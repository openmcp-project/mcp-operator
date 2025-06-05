package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InternalConfigurationComponents defines the components that are part of the internal configuration.
type InternalConfigurationComponents struct {
	APIServer *APIServerInternalConfiguration `json:"apiServer,omitempty"`
}

// InternalConfigurationSpec defines additional configuration for a managedcontrolplane.
type InternalConfigurationSpec struct {
	*InternalCommonConfig `json:",inline"`

	Components InternalConfigurationComponents `json:"components,omitempty"`
}

// +kubebuilder:object:root=true

// InternalConfiguration is the Schema for the InternalConfigurations API
// +kubebuilder:resource:shortName=icfg
type InternalConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec InternalConfigurationSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// InternalConfigurationList contains a list of InternalConfiguration
type InternalConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InternalConfiguration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InternalConfiguration{}, &InternalConfigurationList{})
}
