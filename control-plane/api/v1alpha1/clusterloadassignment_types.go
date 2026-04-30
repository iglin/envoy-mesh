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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ClusterLoadAssignment is the Schema for the clusterloadassignments API
// (envoy.config.endpoint.v3.ClusterLoadAssignment).
type ClusterLoadAssignment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +required
	TargetRef TargetRef `json:"targetRef"`

	// Spec holds the Envoy ClusterLoadAssignment proto serialised as JSON.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	Spec runtime.RawExtension `json:"spec,omitempty"`
}

// GetTargetRef returns the TargetRef for this xDS resource.
func (c *ClusterLoadAssignment) GetTargetRef() TargetRef { return c.TargetRef }

// +kubebuilder:object:root=true

// ClusterLoadAssignmentList contains a list of ClusterLoadAssignment.
type ClusterLoadAssignmentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterLoadAssignment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterLoadAssignment{}, &ClusterLoadAssignmentList{})
}
