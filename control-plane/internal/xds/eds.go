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

package xds

import (
	"encoding/json"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	endpointv3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	discoveryv1 "k8s.io/api/discovery/v1"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// ClusterName extracts the "name" field from a raw Envoy Cluster proto JSON blob.
// Returns "" if the input is empty, malformed, or does not contain a name field.
// This is a pure function with no side effects.
func ClusterName(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var m struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	return m.Name
}

// SynthesizeCLA builds a ClusterLoadAssignment from a list of EndpointSlices for
// the given clusterName. Only Ready endpoints (nil Ready treated as Ready per
// Kubernetes semantics) on the requested port are included.
//
// A non-nil ClusterLoadAssignment is always returned — an empty endpoints list
// is valid and causes Envoy to shed traffic to the cluster (correct during drain).
// This is a pure function with no side effects or shared state.
func SynthesizeCLA(clusterName string, slices []discoveryv1.EndpointSlice, port *int32) *endpointv3.ClusterLoadAssignment {
	cla := &endpointv3.ClusterLoadAssignment{
		ClusterName: clusterName,
	}

	var lbEndpoints []*endpointv3.LbEndpoint
	for i := range slices {
		resolvedPort := resolvePort(&slices[i], port)
		if resolvedPort == 0 {
			continue
		}
		for _, ep := range slices[i].Endpoints {
			// Treat nil Ready as true — Kubernetes spec says absent means Ready.
			if ep.Conditions.Ready != nil && !*ep.Conditions.Ready {
				continue
			}
			for _, addr := range ep.Addresses {
				if addr == "" {
					continue
				}
				lbEndpoints = append(lbEndpoints, &endpointv3.LbEndpoint{
					HostIdentifier: &endpointv3.LbEndpoint_Endpoint{
						Endpoint: &endpointv3.Endpoint{
							Address: &corev3.Address{
								Address: &corev3.Address_SocketAddress{
									SocketAddress: &corev3.SocketAddress{
										Address: addr,
										PortSpecifier: &corev3.SocketAddress_PortValue{
											PortValue: uint32(resolvedPort),
										},
									},
								},
							},
						},
					},
					LoadBalancingWeight: wrapperspb.UInt32(1),
				})
			}
		}
	}

	if len(lbEndpoints) > 0 {
		cla.Endpoints = []*endpointv3.LocalityLbEndpoints{
			{LbEndpoints: lbEndpoints},
		}
	}
	return cla
}

// resolvePort returns the port number to use for a given EndpointSlice.
// If want is nil, the first available port is returned.
// Returns 0 if no matching port is found or all port pointers are nil.
func resolvePort(slice *discoveryv1.EndpointSlice, want *int32) int32 {
	for _, p := range slice.Ports {
		if p.Port == nil {
			continue
		}
		if want == nil {
			return *p.Port
		}
		if *p.Port == *want {
			return *p.Port
		}
	}
	return 0
}
