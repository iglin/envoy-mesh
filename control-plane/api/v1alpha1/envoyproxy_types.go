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
)

const (
	// ConditionConnected is True when the Envoy proxy is actively streaming xDS from the control-plane.
	ConditionConnected = "Connected"
	// ConditionReady is True once at least one xDS snapshot has been pushed to the proxy.
	ConditionReady = "Ready"
)

// EnvoyProxySpec is intentionally empty. The control-plane derives all xDS node identifiers
// from CR metadata: node.id = {name}.{namespace}, node.cluster = {name}.{namespace}.
type EnvoyProxySpec struct{}

// EnvoyProxyStatus defines the observed state of EnvoyProxy.
type EnvoyProxyStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ep
// +kubebuilder:printcolumn:name="Connected",type="string",JSONPath=".status.conditions[?(@.type==\"Connected\")].status"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// EnvoyProxy represents one Envoy proxy instance managed by the control-plane.
type EnvoyProxy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EnvoyProxySpec   `json:"spec,omitempty"`
	Status EnvoyProxyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// EnvoyProxyList contains a list of EnvoyProxy.
type EnvoyProxyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EnvoyProxy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EnvoyProxy{}, &EnvoyProxyList{})
}
