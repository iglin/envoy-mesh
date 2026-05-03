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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// KubernetesServiceRef points to a Kubernetes Service whose live endpoints
// are automatically translated into a ClusterLoadAssignment for this cluster.
// The cluster spec must use type: EDS with ads: {} as the eds_config source.
type KubernetesServiceRef struct {
	// Name of the Kubernetes Service.
	// +required
	Name string `json:"name"`
	// Namespace of the Service. Defaults to the Cluster CR's namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// Port to expose as Envoy endpoints. If omitted, the first port of the Service is used.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port *int32 `json:"port,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Cluster is the Schema for the clusters API (envoy.config.cluster.v3.Cluster).
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	TargetRef TargetRef `json:"targetRef"`

	// KubernetesServiceRef auto-discovers endpoints from a Kubernetes Service and
	// synthesises a ClusterLoadAssignment in memory at reconcile time.
	// Mutually exclusive with a manual ClusterLoadAssignment CR for the same cluster_name.
	// +optional
	KubernetesServiceRef *KubernetesServiceRef `json:"kubernetesServiceRef,omitempty"`

	// Spec holds the Envoy Cluster proto serialised as JSON.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Spec runtime.RawExtension `json:"spec,omitempty"`
}

// GetTargetRef returns the TargetRef for this xDS resource.
func (c *Cluster) GetTargetRef() TargetRef { return c.TargetRef }

// +kubebuilder:object:root=true

// ClusterList contains a list of Cluster.
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
