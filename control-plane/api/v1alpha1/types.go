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

// TargetRef references the EnvoyProxy CR that an xDS resource CR targets.
type TargetRef struct {
	// Name of the EnvoyProxy CR.
	// +required
	Name string `json:"name"`
	// Namespace of the EnvoyProxy CR. Defaults to the xDS resource's own namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}
