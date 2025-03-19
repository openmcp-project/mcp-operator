package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CloudOrchestratorConfiguration contains the configuration for setting up the CloudOrchestrator component in a ManagedControlPlane.
type CloudOrchestratorConfiguration struct {
	// Crossplane defines the configuration for setting up the Crossplane component in a ManagedControlPlane.
	// +kubebuilder:validation:Optional
	Crossplane *CrossplaneConfig `json:"crossplane,omitempty"`

	// BTPServiceOperator defines the configuration for setting up the BTPServiceOperator component in a ManagedControlPlane.
	// +kubebuilder:validation:Optional
	BTPServiceOperator *BTPServiceOperatorConfig `json:"btpServiceOperator,omitempty"`

	// ExternalSecretsOperator defines the configuration for setting up the ExternalSecretsOperator component in a ManagedControlPlane.
	// +kubebuilder:validation:Optional
	ExternalSecretsOperator *ExternalSecretsOperatorConfig `json:"externalSecretsOperator,omitempty"`

	// Kyverno defines the configuration for setting up the Kyverno component in a ManagedControlPlane.
	// +kubebuilder:validation:Optional
	Kyverno *KyvernoConfig `json:"kyverno,omitempty"`

	// Flux defines the configuration for setting up the Flux component in a ManagedControlPlane.
	// +kubebuilder:validation:Optional
	Flux *FluxConfig `json:"flux,omitempty"`
}

// CloudOrchestratorSpec defines the desired state of CloudOrchestrator
type CloudOrchestratorSpec struct {
	CloudOrchestratorConfiguration `json:",inline"`
}

// ExternalCloudOrchestratorStatus contains the status of the CloudOrchestrator component.
type ExternalCloudOrchestratorStatus struct {
}

// CloudOrchestratorStatus defines the observed state of CloudOrchestrator
type CloudOrchestratorStatus struct {
	CommonComponentStatus            `json:",inline"`
	*ExternalCloudOrchestratorStatus `json:",inline"`

	// Number of enabled components.
	// +kubebuilder:validation:Optional
	ComponentsEnabled int `json:"componentsEnabled"`

	// Number of healthy components.
	// +kubebuilder:validation:Optional
	ComponentsHealthy int `json:"componentsHealthy"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:shortName=co
// +kubebuilder:printcolumn:name="Successfully_Reconciled",type=string,JSONPath=`.status.conditions[?(@.type=="CloudOrchestratorReconciliation")].status`
// +kubebuilder:printcolumn:name="Deleted",type="date",JSONPath=".metadata.deletionTimestamp"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// CloudOrchestrator is the Schema for the internal CloudOrchestrator API
type CloudOrchestrator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudOrchestratorSpec   `json:"spec,omitempty"`
	Status CloudOrchestratorStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CloudOrchestratorList contains a list of CloudOrchestrator
type CloudOrchestratorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudOrchestrator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CloudOrchestrator{}, &CloudOrchestratorList{})
}

// CrossplaneConfig defines the configuration of Crossplane
type CrossplaneConfig struct {
	// The Version of Crossplane to install.
	// +kubebuilder:validation:Required
	Version string `json:"version"`

	Providers []*CrossplaneProviderConfig `json:"providers,omitempty"`
}

// BTPServiceOperatorConfig defines the configuration of BTPServiceOperator
type BTPServiceOperatorConfig struct {
	// The Version of BTP Service Operator to install.
	// +kubebuilder:validation:Required
	Version string `json:"version"`
}

// ExternalSecretsOperatorConfig defines the configuration of ExternalSecretsOperator
type ExternalSecretsOperatorConfig struct {
	// The Version of External Secrets Operator to install.
	// +kubebuilder:validation:Required
	Version string `json:"version"`
}

// KyvernoConfig defines the configuration of Kyverno
type KyvernoConfig struct {
	// The Version of Kyverno to install.
	// +kubebuilder:validation:Required
	Version string `json:"version"`
}

// FluxConfig defines the configuration of Flux
type FluxConfig struct {
	// The Version of Flux to install.
	// +kubebuilder:validation:Required
	Version string `json:"version"`
}

type CrossplaneProviderConfig struct {
	// Name of the provider.
	// Using a well-known name will automatically configure the "package" field.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Version of the provider to install.
	// +kubebuilder:validation:Required
	Version string `json:"version"`
}
