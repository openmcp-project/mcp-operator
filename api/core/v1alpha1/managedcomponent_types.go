// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ManagedComponentSpec defines the desired state of ManagedComponent.
type ManagedComponentSpec struct{}

// ManagedComponentStatus defines the observed state of ManagedComponent.
type ManagedComponentStatus struct {
	Versions []string `json:"versions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".spec.name"
// +kubebuilder:printcolumn:name="Versions",type="string",JSONPath=".status.versions"

// ManagedComponent is the Schema for the managedcomponents API.
type ManagedComponent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedComponentSpec   `json:"spec,omitempty"`
	Status ManagedComponentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManagedComponentList contains a list of ManagedComponent.
type ManagedComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedComponent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagedComponent{}, &ManagedComponentList{})
}
