package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// Well-known oidc-login parameters

	OIDCParameterIssuerURL    = "oidc-issuer-url"
	OIDCParameterClientID     = "oidc-client-id"
	OIDCParameterClientSecret = "oidc-client-secret"
	OIDCParameterExtraScope   = "oidc-extra-scope"
	OIDCParameterPKCEMethod   = "oidc-pkce-method"
	OIDCParameterGrantType    = "grant-type"

	OIDCDefaultExtraScopes = "offline_access,email,profile"
	OIDCDefaultPKCEMethod  = "auto"
	OIDCDefaultGrantType   = "auto"
)

// AuthenticationConfiguration contains the configuration for the enabled OpenID Connect identity providers
type AuthenticationConfiguration struct {
	// +kubebuilder:validation:Optional
	EnableSystemIdentityProvider *bool `json:"enableSystemIdentityProvider"`
	// +kubebuilder:validation:Optional
	IdentityProviders []IdentityProvider `json:"identityProviders,omitempty"`
}

// AuthenticationSpec contains the specification for the authentication component
type AuthenticationSpec struct {
	AuthenticationConfiguration `json:",inline"`
}

// ExternalAuthenticationStatus contains the status of the  authentication component.
type ExternalAuthenticationStatus struct {
	// UserAccess reference the secret containing the kubeconfig
	// for the APIServer which is to be used by the customer.
	// +optional
	UserAccess *SecretReference `json:"access,omitempty"`
}

// AuthenticationStatus contains the status of the authentication component
type AuthenticationStatus struct {
	CommonComponentStatus         `json:",inline"`
	*ExternalAuthenticationStatus `json:",inline"`
}

// IdentityProvider contains the configuration for an OpenID Connect identity provider
type IdentityProvider struct {
	// Name is the name of the identity provider.
	// The name must be unique among all identity providers.
	// The name must only contain lowercase letters.
	// The length must not exceed 63 characters.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-z]+$`
	Name string `json:"name"`
	// IssuerURL is the issuer URL of the identity provider.
	// +kubebuilder:validation:Required
	IssuerURL string `json:"issuerURL"`
	// ClientID is the client ID of the identity provider.
	// +kubebuilder:validation:Required
	ClientID string `json:"clientID"`
	// UsernameClaim is the claim that contains the username.
	// +kubebuilder:validation:Required
	UsernameClaim string `json:"usernameClaim"`
	// GroupsClaim is the claim that contains the groups.
	// +kubebuilder:validation:Optional
	GroupsClaim string `json:"groupsClaim"`
	// CABundle: When set, the OpenID server's certificate will be verified by one of the authorities in the bundle.
	// Otherwise, the host's root CA set will be used.
	// +kubebuilder:validation:Optional
	CABundle string `json:"caBundle,omitempty"`
	// SigningAlgs is the list of allowed JOSE asymmetric signing algorithms.
	// +kubebuilder:validation:Optional
	SigningAlgs []string `json:"signingAlgs,omitempty"`
	// RequiredClaims is a map of required claims. If set, the identity provider must provide these claims in the ID token.
	// +kubebuilder:validation:Optional
	RequiredClaims map[string]string `json:"requiredClaims,omitempty"`

	// ClientAuthentication contains configuration for OIDC clients
	// +kubebuilder:validation:Optional
	ClientConfig ClientAuthenticationConfig `json:"clientConfig,omitempty"`
}

// ClientAuthenticationConfig contains configuration for OIDC clients
type ClientAuthenticationConfig struct {
	// ClientSecret is a references to a secret containing the client secret.
	// The client secret will be added to the generated kubeconfig with the "--oidc-client-secret" flag.
	// +kubebuilder:validation:Optional
	ClientSecret *LocalSecretReference `json:"clientSecret,omitempty"`
	// ExtraConfig is added to the client configuration in the kubeconfig.
	// Can either be a single string value, a list of string values or no value.
	// Must not contain any of the following keys:
	// - "client-id"
	// - "client-secret"
	// - "issuer-url"
	//
	// +kubebuilder:validation:Optional
	ExtraConfig map[string]SingleOrMultiStringValue `json:"extraConfig,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Authentication is the Schema for the authentication API
// +kubebuilder:resource:shortName=auth
// +kubebuilder:printcolumn:name="Successfully_Reconciled",type=string,JSONPath=`.status.conditions[?(@.type=="AuthenticationReconciliation")].status`
// +kubebuilder:printcolumn:name="Deleted",type="date",JSONPath=".metadata.deletionTimestamp"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Authentication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthenticationSpec   `json:"spec,omitempty"`
	Status AuthenticationStatus `json:"status,omitempty"`
}

// IsSystemIdentityProviderEnabled returns true if the system identity provider is enabled
func (a *Authentication) IsSystemIdentityProviderEnabled() bool {
	return a.Spec.EnableSystemIdentityProvider != nil && *a.Spec.EnableSystemIdentityProvider
}

// +kubebuilder:object:root=true

// AuthenticationList contains the list of authentications
type AuthenticationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Authentication `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Authentication{}, &AuthenticationList{})
}
