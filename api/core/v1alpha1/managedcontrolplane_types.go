package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ManagedControlPlaneComponents contains the configuration for the components of a ManagedControlPlane.
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.apiServer)|| has(self.apiServer)",message="apiServer is required once set"
type ManagedControlPlaneComponents struct {
	// +kubebuilder:default={"type":"GardenerDedicated"}
	APIServer *APIServerConfiguration `json:"apiServer,omitempty"`

	Landscaper *LandscaperConfiguration `json:"landscaper,omitempty"`

	CloudOrchestratorConfiguration `json:",inline"`
}

// ManagedControlPlaneSpec defines the desired state of ManagedControlPlane.
type ManagedControlPlaneSpec struct {
	// DisabledComponents contains a list of component types.
	// The resources for these components will still be generated, but they will get the ignore operation annotation, so they should not be processed by their respective controllers.
	DisabledComponents []ComponentType `json:"disabledComponents,omitempty"`

	// CommonConfig contains configuration that is passed to all component controllers.
	*CommonConfig `json:",inline"`

	// Authentication contains the configuration for the enabled OpenID Connect identity providers
	Authentication *AuthenticationConfiguration `json:"authentication,omitempty"`

	// Authorization contains the configuration of the subjects assigned to control plane roles
	Authorization *AuthorizationConfiguration `json:"authorization,omitempty"`

	// Components contains the configuration for Components like APIServer, Landscaper, CloudOrchestrator.
	Components ManagedControlPlaneComponents `json:"components"`
}

// ManagedControlPlaneComponentsStatus contains the status of the components of a ManagedControlPlane.
type ManagedControlPlaneComponentsStatus struct {
	APIServer *ExternalAPIServerStatus `json:"apiServer,omitempty"`

	Landscaper *ExternalLandscaperStatus `json:"landscaper,omitempty"`

	CloudOrchestrator *ExternalCloudOrchestratorStatus `json:"cloudOrchestrator,omitempty"`

	Authentication *ExternalAuthenticationStatus `json:"authentication,omitempty"`

	Authorization *ExternalAuthorizationStatus `json:"authorization,omitempty"`
}

// ManagedControlPlaneStatus defines the observed state of ManagedControlPlane.
type ManagedControlPlaneStatus struct {
	ManagedControlPlaneMetaStatus `json:",inline"`

	// Conditions collects the conditions of all components.
	Conditions []ManagedControlPlaneComponentCondition `json:"conditions,omitempty"`

	Components ManagedControlPlaneComponentsStatus `json:"components,omitempty"`
}

type ManagedControlPlaneComponentCondition struct {
	ComponentCondition `json:",inline"`

	// ManagedBy contains the information which component manages this condition.
	ManagedBy ComponentType `json:"managedBy"`
}

type ManagedControlPlaneMetaStatus struct {
	// ObservedGeneration is the last generation of this resource that has successfully been reconciled.
	ObservedGeneration int64 `json:"observedGeneration"`

	// Status is the current status of the ManagedControlPlane.
	// It is "Deleting" if the ManagedControlPlane is being deleted.
	// It is "Ready" if all conditions are true, and "Not Ready" otherwise.
	Status MCPStatus `json:"status"`

	// Message contains an optional message.
	// +optional
	Message string `json:"message,omitempty"`
}

// MCPStatus is a type for the status of a ManagedControlPlane.
// Use NewMCPStatus to create a new MCPStatus, or use one of the predefined constants.
type MCPStatus string

const (
	// MCPStatusReady indicates that the ManagedControlPlane is ready.
	MCPStatusReady MCPStatus = "Ready"

	// MCPStatusNotReady indicates that the ManagedControlPlane is not ready.
	MCPStatusNotReady MCPStatus = "Not Ready"

	// MCPStatusDeleting indicates that the ManagedControlPlane is being deleted.
	MCPStatusDeleting MCPStatus = "Deleting"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ManagedControlPlane is the Schema for the ManagedControlPlane API
// +kubebuilder:resource:shortName=mcp
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:validation:XValidation:rule="size(self.metadata.name) <= 36",message="name must not be longer than 36 characters"
type ManagedControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedControlPlaneSpec   `json:"spec,omitempty"`
	Status ManagedControlPlaneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ManagedControlPlaneList contains a list of ManagedControlPlane
type ManagedControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ManagedControlPlane{}, &ManagedControlPlaneList{})
}
