package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type APIServerType string

const (
	// Gardener is the APIServerType for a workerless shoot cluster.
	Gardener APIServerType = "Gardener"

	// GardenerDedicated is the APIServerType for a cluster with worker nodes.
	GardenerDedicated APIServerType = "GardenerDedicated"
)

// APIServerConfiguration contains the configuration which is required for setting up a k8s cluster to be used as APIServer.
type APIServerConfiguration struct {
	// Type is the type of APIServer. This determines which other configuration fields need to be specified.
	// Valid values are:
	// - Gardener
	// - GardenerDedicated
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="type is immutable"
	// +kubebuilder:validation:Enum=Gardener;GardenerDedicated
	// +kubebuilder:default="GardenerDedicated"
	Type APIServerType `json:"type"`

	// GardenerConfig contains configuration for a Gardener APIServer.
	// Must be set if type is 'Gardener', is ignored otherwise.
	// +optional
	GardenerConfig *GardenerConfiguration `json:"gardener,omitempty"`
}

type APIServerInternalConfiguration struct {
	// GardenerConfig contains internal configuration for a Gardener APIServer.
	// +optional
	GardenerConfig *GardenerInternalConfiguration `json:"gardener,omitempty"`
}

// APIServerSpec contains the APIServer configuration and potentially other fields which should not be exposed to the customer.
type APIServerSpec struct {
	APIServerConfiguration `json:",inline"`

	// Internal contains the parts of the configuration which are not exposed to the customer.
	// It would be nice to have this as an inline field, but since both APIServerConfiguration and APIServerInternalConfiguration
	// contain a field 'gardener', this would clash.
	// +optional
	Internal *APIServerInternalConfiguration `json:"internal,omitempty"`

	// DesiredRegion is part of the common configuration.
	// If specified, it will be used to determine the region for the created cluster.
	// +optional
	DesiredRegion *RegionSpecification `json:"desiredRegion"`
}

// ExternalAPIServerStatus contains the status of the API server / ManagedControlPlane cluster. The Kuberenetes can act as an OIDC
// compatible provider in a sense that they serve OIDC issuer endpoint URL so that other system can validate tokens that have been
// issued by the external party.
type ExternalAPIServerStatus struct {
	// Endpoint represents the Kubernetes API server endpoint
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// ServiceAccountIssuer represents the OpenIDConnect issuer URL that can be used to verify service account tokens.
	// +optional
	ServiceAccountIssuer string `json:"serviceAccountIssuer,omitempty"`
}

// APIServerStatus contains the APIServer status and potentially other fields which should not be exposed to the customer.
type APIServerStatus struct {
	CommonComponentStatus `json:",inline"`

	// ExternalAPIServerStatus contains the status of the external API server
	ExternalAPIServerStatus `json:",inline"`

	// AdminAccess is an admin kubeconfig for accessing the API server.
	// +optional
	AdminAccess *APIServerAccess `json:"adminAccess,omitempty"`

	// GardenerStatus contains status if the type is 'Gardener'.
	// +optional
	GardenerStatus *GardenerStatus `json:"gardener,omitempty"`
}

// APIServerAccess contains access information for the API server.
// Usually a kubeconfig, optional some metadata.
type APIServerAccess struct {
	// Kubeconfig is the kubeconfig for accessing the APIServer cluster.
	Kubeconfig string `json:"kubeconfig,omitempty"`

	// CreationTimestamp is the time when this access was created.
	// +optional
	CreationTimestamp *metav1.Time `json:"creationTimestamp,omitempty"`

	// ExpirationTimestamp is the time until the access loses its validity.
	// +optional
	ExpirationTimestamp *metav1.Time `json:"expirationTimestamp,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// APIServer is the Schema for the APIServer API
// +kubebuilder:resource:shortName=as
// +kubebuilder:printcolumn:name="Successfully_Reconciled",type=string,JSONPath=`.status.conditions[?(@.type=="APIServerReconciliation")].status`
// +kubebuilder:printcolumn:name="Deleted",type="date",JSONPath=".metadata.deletionTimestamp"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type APIServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   APIServerSpec   `json:"spec,omitempty"`
	Status APIServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// APIServerList contains a list of APIServer
type APIServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []APIServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&APIServer{}, &APIServerList{})
}
