/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	meshv1alpha1 "github.com/iglin/envoy-mesh/control-plane/api/v1alpha1"
	"github.com/iglin/envoy-mesh/control-plane/internal/xds"
)

// EnvoyProxyReconciler reconciles EnvoyProxy objects and all xDS resource CRs
// that reference them. It builds one xDS snapshot per EnvoyProxy and pushes it
// to the shared snapshot cache.
type EnvoyProxyReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	XDSServer *xds.Server
}

// +kubebuilder:rbac:groups=mesh.envoy.io,resources=envoyproxies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mesh.envoy.io,resources=envoyproxies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=mesh.envoy.io,resources=envoyproxies/finalizers,verbs=update
// +kubebuilder:rbac:groups=mesh.envoy.io,resources=listeners,verbs=get;list;watch
// +kubebuilder:rbac:groups=mesh.envoy.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=mesh.envoy.io,resources=routeconfigurations,verbs=get;list;watch
// +kubebuilder:rbac:groups=mesh.envoy.io,resources=scopedrouteconfigurations,verbs=get;list;watch
// +kubebuilder:rbac:groups=mesh.envoy.io,resources=clusterloadassignments,verbs=get;list;watch

func (r *EnvoyProxyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var ep meshv1alpha1.EnvoyProxy
	if err := r.Get(ctx, req.NamespacedName, &ep); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	nodeID := fmt.Sprintf("%s.%s", ep.Name, ep.Namespace)
	log.Info("Reconciling EnvoyProxy", "nodeID", nodeID)

	listeners, lVers, err := r.collectListeners(ctx, ep.Namespace, ep.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("collect listeners: %w", err)
	}
	clusters, cVers, err := r.collectClusters(ctx, ep.Namespace, ep.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("collect clusters: %w", err)
	}
	routes, rVers, err := r.collectRoutes(ctx, ep.Namespace, ep.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("collect routes: %w", err)
	}
	scopedRoutes, srVers, err := r.collectScopedRoutes(ctx, ep.Namespace, ep.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("collect scoped routes: %w", err)
	}
	endpoints, eVers, err := r.collectEndpoints(ctx, ep.Namespace, ep.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("collect endpoints: %w", err)
	}

	version := computeVersion(lVers, cVers, rVers, srVers, eVers)

	snap, err := xds.BuildSnapshot(version, listeners, clusters, routes, scopedRoutes, endpoints)
	if err != nil {
		r.setCondition(ctx, &ep, meshv1alpha1.ConditionReady, metav1.ConditionFalse, "SnapshotBuildFailed", err.Error())
		return ctrl.Result{}, fmt.Errorf("build snapshot: %w", err)
	}

	if err := r.XDSServer.Cache.SetSnapshot(ctx, nodeID, snap); err != nil {
		r.setCondition(ctx, &ep, meshv1alpha1.ConditionReady, metav1.ConditionFalse, "SnapshotSetFailed", err.Error())
		return ctrl.Result{}, fmt.Errorf("set snapshot: %w", err)
	}

	log.Info("Snapshot updated", "nodeID", nodeID, "version", version)

	connected := r.XDSServer.IsConnected(nodeID)
	connStatus := metav1.ConditionFalse
	if connected {
		connStatus = metav1.ConditionTrue
	}
	r.setCondition(ctx, &ep, meshv1alpha1.ConditionConnected, connStatus, "StreamStatus", "")
	r.setCondition(ctx, &ep, meshv1alpha1.ConditionReady, metav1.ConditionTrue, "SnapshotReady",
		fmt.Sprintf("version=%s", version))

	return ctrl.Result{}, nil
}

// targetMatches reports whether a TargetRef points to the given proxy name/namespace.
func targetMatches(ref meshv1alpha1.TargetRef, itemNS, proxyNS, proxyName string) bool {
	ns := ref.Namespace
	if ns == "" {
		ns = itemNS
	}
	return ref.Name == proxyName && ns == proxyNS
}

func (r *EnvoyProxyReconciler) collectListeners(ctx context.Context, proxyNS, proxyName string) ([][]byte, []string, error) {
	var list meshv1alpha1.ListenerList
	if err := r.List(ctx, &list); err != nil {
		return nil, nil, err
	}
	var specs [][]byte
	var versions []string
	for _, item := range list.Items {
		if targetMatches(item.TargetRef, item.Namespace, proxyNS, proxyName) {
			specs = append(specs, item.Spec.Raw)
			versions = append(versions, item.ResourceVersion)
		}
	}
	return specs, versions, nil
}

func (r *EnvoyProxyReconciler) collectClusters(ctx context.Context, proxyNS, proxyName string) ([][]byte, []string, error) {
	var list meshv1alpha1.ClusterList
	if err := r.List(ctx, &list); err != nil {
		return nil, nil, err
	}
	var specs [][]byte
	var versions []string
	for _, item := range list.Items {
		if targetMatches(item.TargetRef, item.Namespace, proxyNS, proxyName) {
			specs = append(specs, item.Spec.Raw)
			versions = append(versions, item.ResourceVersion)
		}
	}
	return specs, versions, nil
}

func (r *EnvoyProxyReconciler) collectRoutes(ctx context.Context, proxyNS, proxyName string) ([][]byte, []string, error) {
	var list meshv1alpha1.RouteConfigurationList
	if err := r.List(ctx, &list); err != nil {
		return nil, nil, err
	}
	var specs [][]byte
	var versions []string
	for _, item := range list.Items {
		if targetMatches(item.TargetRef, item.Namespace, proxyNS, proxyName) {
			specs = append(specs, item.Spec.Raw)
			versions = append(versions, item.ResourceVersion)
		}
	}
	return specs, versions, nil
}

func (r *EnvoyProxyReconciler) collectScopedRoutes(ctx context.Context, proxyNS, proxyName string) ([][]byte, []string, error) {
	var list meshv1alpha1.ScopedRouteConfigurationList
	if err := r.List(ctx, &list); err != nil {
		return nil, nil, err
	}
	var specs [][]byte
	var versions []string
	for _, item := range list.Items {
		if targetMatches(item.TargetRef, item.Namespace, proxyNS, proxyName) {
			specs = append(specs, item.Spec.Raw)
			versions = append(versions, item.ResourceVersion)
		}
	}
	return specs, versions, nil
}

func (r *EnvoyProxyReconciler) collectEndpoints(ctx context.Context, proxyNS, proxyName string) ([][]byte, []string, error) {
	var list meshv1alpha1.ClusterLoadAssignmentList
	if err := r.List(ctx, &list); err != nil {
		return nil, nil, err
	}
	var specs [][]byte
	var versions []string
	for _, item := range list.Items {
		if targetMatches(item.TargetRef, item.Namespace, proxyNS, proxyName) {
			specs = append(specs, item.Spec.Raw)
			versions = append(versions, item.ResourceVersion)
		}
	}
	return specs, versions, nil
}

// computeVersion hashes all resource versions and raw specs to produce a stable snapshot version.
func computeVersion(groups ...[]string) string {
	h := sha256.New()
	for _, group := range groups {
		for _, v := range group {
			_, _ = fmt.Fprint(h, v)
		}
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (r *EnvoyProxyReconciler) setCondition(
	ctx context.Context,
	ep *meshv1alpha1.EnvoyProxy,
	condType string,
	status metav1.ConditionStatus,
	reason, msg string,
) {
	meta.SetStatusCondition(&ep.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: ep.Generation,
	})
	ep.Status.ObservedGeneration = ep.Generation
	if err := r.Status().Update(ctx, ep); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to update EnvoyProxy status", "condition", condType)
	}
}

// mapXDSResourceToProxy maps any xDS resource CR to the EnvoyProxy it references,
// enqueuing a reconcile request for that proxy.
func (r *EnvoyProxyReconciler) mapXDSResourceToProxy(ctx context.Context, obj client.Object) []reconcile.Request {
	type targetReffer interface {
		GetTargetRef() meshv1alpha1.TargetRef
	}
	tr, ok := obj.(targetReffer)
	if !ok {
		return nil
	}
	ref := tr.GetTargetRef()
	ns := ref.Namespace
	if ns == "" {
		ns = obj.GetNamespace()
	}
	return []reconcile.Request{{
		NamespacedName: types.NamespacedName{Name: ref.Name, Namespace: ns},
	}}
}

// SetupWithManager registers the controller and watches all xDS resource types so
// that any change to them triggers an EnvoyProxy reconcile.
func (r *EnvoyProxyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	mapFn := handler.EnqueueRequestsFromMapFunc(r.mapXDSResourceToProxy)
	return ctrl.NewControllerManagedBy(mgr).
		For(&meshv1alpha1.EnvoyProxy{}).
		Watches(&meshv1alpha1.Listener{}, mapFn).
		Watches(&meshv1alpha1.Cluster{}, mapFn).
		Watches(&meshv1alpha1.RouteConfiguration{}, mapFn).
		Watches(&meshv1alpha1.ScopedRouteConfiguration{}, mapFn).
		Watches(&meshv1alpha1.ClusterLoadAssignment{}, mapFn).
		Named("envoyproxy").
		Complete(r)
}
