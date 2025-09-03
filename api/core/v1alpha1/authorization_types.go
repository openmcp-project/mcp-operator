package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// RoleBindingRoleAdmin is the role for the admin
	RoleBindingRoleAdmin = "admin"
	// RoleBindingRoleView is the role for the viewer
	RoleBindingRoleView = "view"

	// AdminNamespaceScopeRole is the role for the admin with namespace scope
	AdminNamespaceScopeRole = "openmcp:admin"
	// AdminClusterScopeRole is the role for the admin with cluster scope
	AdminClusterScopeRole = "openmcp:admin:clusterscoped"
	// AdminNamespaceScopeStandardRulesRole is the role for the admin with namespace scope and standard rules
	AdminNamespaceScopeStandardRulesRole = "openmcp:aggregate-to-admin"
	// AdminClusterScopeStandardRulesRole is the role for the admin with cluster scope and standard rules
	AdminClusterScopeStandardRulesRole = "openmcp:clusterscoped:aggregate-to-admin"
	// AdminNamespaceScopeMatchLabel is the aggregation label for the admin with namespace scope
	AdminNamespaceScopeMatchLabel = BaseDomain + "/aggregate-to-admin"
	// AdminClusterScopeMatchLabel is the aggregation label for the admin with cluster scope
	AdminClusterScopeMatchLabel = BaseDomain + "/aggregate-to-admin-clusterscoped"

	// ViewNamespaceScopeRole is the role for the viewer with namespace scope
	ViewNamespaceScopeRole = "openmcp:view"
	// ViewClusterScopeRole is the role for the viewer with cluster scope
	ViewClusterScopeRole = "openmcp:view:clusterscoped"
	// ViewNamespaceScopeStandardRulesRole is the role for the viewer with namespace scope and standard rules
	ViewNamespaceScopeStandardRulesRole = "openmcp:aggregate-to-view"
	// ViewClusterScopeStandardClusterRole is the role for the viewer with cluster scope and standard rules
	ViewClusterScopeStandardClusterRole = "openmcp:clusterscoped:aggregate-to-view"
	// ViewNamespaceScopeMatchLabel is the aggregation label for the viewer with namespace scope
	ViewNamespaceScopeMatchLabel = BaseDomain + "/aggregate-to-view"
	// ViewClusterScopeMatchLabel is the aggregation label for the viewer with cluster scope
	ViewClusterScopeMatchLabel = BaseDomain + "/aggregate-to-view-clusterscoped"

	// AdminClusterRoleBinding is the cluster role binding for the admin with cluster scope
	AdminClusterRoleBinding = "openmcp:admin"
	// AdminRoleBinding is the role binding for the admin with namespace scope
	AdminRoleBinding = "openmcp:admin"
	// ViewClusterRoleBinding is the cluster role binding for the viewer with cluster scope
	ViewClusterRoleBinding = "openmcp:view"
	// ViewRoleBinding is the role binding for the viewer with namespace scope
	ViewRoleBinding = "openmcp:view"

	// ClusterAdminRoleBinding is the name of the role binding for the cluster admin
	ClusterAdminRoleBinding = "openmcp:cluster-admin"
	// ClusterAdminRole is the name of the role for the cluster admin
	ClusterAdminRole = "cluster-admin"
)

// AuthorizationConfiguration contains the configuration of the subjects assigned to control plane roles
type AuthorizationConfiguration struct {
	// RoleBindings is a list of role bindings
	RoleBindings []RoleBinding `json:"roleBindings"`
}

// GetRoleForName returns the role for the given role name or nil if the role does not exist.
// If multiple roles with the same name exist, their subject lists are aggregated.
func (ac *AuthorizationConfiguration) GetRoleForName(roleName string) *RoleBinding {
	var res *RoleBinding
	for _, rb := range ac.RoleBindings {
		if rb.Role == roleName {
			if res == nil {
				res = &RoleBinding{
					Role: roleName,
				}
			}
			res.Subjects = append(res.Subjects, rb.Subjects...)
		}
	}
	return res
}

// RoleBinding contains the role and the subjects assigned to the role
type RoleBinding struct {
	// Role is the name of the role
	// +kubebuilder:validation:Enum=admin;view
	Role string `json:"role"`
	// Subjects is a list of subjects assigned to the role
	Subjects []Subject `json:"subjects"`
}

// Subject describes an object that is assigned to a role and
// which can be used to authenticate against the control plane.
type Subject struct {
	// Kind is the kind of the subject
	// +kubebuilder:validation:Enum=ServiceAccount;User;Group
	Kind string `json:"kind"`
	// APIGroup is the API group of the subject
	// +optional
	APIGroup string `json:"apiGroup,omitempty"`
	// Name is the name of the subject
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Namespace is the namespace of the subject
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// AuthorizationSpec contains the specification for the authorization component
type AuthorizationSpec struct {
	AuthorizationConfiguration `json:",inline"`
}

// ExternalAuthorizationStatus contains the status of the external authorization component
type ExternalAuthorizationStatus struct {
}

// AuthorizationStatus contains the status of the authorization component
type AuthorizationStatus struct {
	CommonComponentStatus `json:",inline"`
	// ExternalAuthorizationStatus contains the status of the external authorization component
	ExternalAuthorizationStatus `json:",inline"`

	// UserNamespaces is a list of namespaces that have been created by the user and
	// must be managed by the authorization component.
	UserNamespaces []string `json:"userNamespaces,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Authorization is the Schema for the authorization API
// +kubebuilder:resource:shortName=authz
// +kubebuilder:printcolumn:name="Successfully_Reconciled",type=string,JSONPath=`.status.conditions[?(@.type=="AuthorizationReconciliation")].status`
// +kubebuilder:printcolumn:name="Deleted",type="date",JSONPath=".metadata.deletionTimestamp"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
type Authorization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthorizationSpec   `json:"spec,omitempty"`
	Status AuthorizationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AuthorizationList contains the list of authorizations
type AuthorizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Authorization `json:"items"`
}

// ClusterAdminSpec contains the specification for the cluster admin
type ClusterAdminSpec struct {
	Subjects []Subject `json:"subjects"`
}

// ClusterAdminStatus contains the status of the cluster admin
type ClusterAdminStatus struct {
	// Active is set to true if the subjects of the cluster admin are assigned the cluster-admin role
	Active bool `json:"active"`
	// ActivationTime is the time when the cluster admin was activated
	// +optional
	Activated *metav1.Time `json:"activationTime,omitempty"`
	// ExpirationTime is the time when the cluster admin will expire
	// +optional
	Expiration *metav1.Time `json:"expirationTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ClusterAdmin is the Schema for the cluster admin API
// +kubebuilder:resource:shortName=clas
// +kubebuilder:printcolumn:name="Active",type=string,JSONPath=`.status.active`
// +kubebuilder:printcolumn:name="Activated",type="date",JSONPath=".status.activationTime"
// +kubebuilder:printcolumn:name="Expiration",type="string",JSONPath=".status.expirationTime"
type ClusterAdmin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterAdminSpec   `json:"spec,omitempty"`
	Status ClusterAdminStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterAdminList contains the list of cluster admins
type ClusterAdminList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterAdmin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Authorization{}, &AuthorizationList{}, &ClusterAdmin{}, &ClusterAdminList{})
}
