package config

import (
	"regexp"
	"strings"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

// AuthorizationConfig contains the configuration for the authorization controller.
type AuthorizationConfig struct {
	// Admin contains the configuration for the admin role.
	Admin RoleConfig `json:"admin,omitempty"`
	// View contains the configuration for the view role.
	View RoleConfig `json:"view,omitempty"`

	// ProtectedNamespaces contains the list of namespaces that are protected from being modified by the user.
	ProtectedNamespaces []ProtectedNamespace `json:"protectedNamespaces,omitempty"`

	// ClusterAdmin contains the configuration for the cluster admin role.
	ClusterAdmin ClusterAdmin `json:"clusterAdmin,omitempty"`
}

// RoleConfig contains the configuration for a role.
type RoleConfig struct {
	// AdditionalSubjects contains the additional subjects for the role.
	// They are added to a MCP alongside the subjects specified by the user.
	AdditionalSubjects []rbacv1.Subject `json:"additionalSubjects,omitempty"`
	// NamespaceScoped contains the configuration for the namespace scoped rules of the role.
	NamespaceScoped RulesConfig `json:"namespaceScoped,omitempty"`
	// ClusterScoped contains the configuration for the cluster scoped rules of the role.
	ClusterScoped RulesConfig `json:"clusterScoped,omitempty"`
}

// RulesConfig contains the configuration for the rules of a role.
type RulesConfig struct {
	// Labels are added to the `ClusterRole` that defines the common rules for a user.
	Labels map[string]string `json:"labels,omitempty"`
	// ClusterRoleSelectors define label selector which aggregate specific `Cluster` to the common `ClusterRole`.
	ClusterRoleSelectors []metav1.LabelSelector `json:"clusterRoleSelectors,omitempty"`
	// Rules specifies the rules for the role.
	Rules []rbacv1.PolicyRule `json:"rules,omitempty"`
}

// ProtectedNamespace contains the configuration for a protected namespace.
// If any of the non-empty fields is matched, the namespace is considered protected.
// The ordering of the matching is as follows:
// 1. Exact
// 2. Prefix
// 3. Postfix
// 4. Pattern
type ProtectedNamespace struct {
	// Exact is the exact namespace name.
	Exact string `json:"exact,omitempty"`
	// Prefix is the prefix of the namespace name.
	Prefix string `json:"prefix,omitempty"`
	// Postfix is the postfix of the namespace name.
	Postfix string `json:"postfix,omitempty"`
	// Pattern is the pattern of the namespace name.
	Pattern string `json:"pattern,omitempty"`

	// CompiledPattern is the compiled pattern of the namespace name.
	// Not serialized.
	CompiledPattern *regexp.Regexp `json:"-"`
}

// ClusterAdmin contains the configuration for the cluster admin role.
type ClusterAdmin struct {
	// ActiveDuration is the duration for which the cluster admin role is active.
	ActiveDuration metav1.Duration `json:"activeDuration,omitempty"`
}

// SetDefaults sets the default values for the authorization configuration when not set.
func (ac *AuthorizationConfig) SetDefaults() {
	// Admin Section
	admin := &ac.Admin
	adminNamespaceScoped := &admin.NamespaceScoped
	adminClusterScoped := &admin.ClusterScoped

	if adminNamespaceScoped.ClusterRoleSelectors == nil {
		adminNamespaceScoped.ClusterRoleSelectors = make([]metav1.LabelSelector, 0)
	}

	adminNamespaceScoped.ClusterRoleSelectors = append(adminNamespaceScoped.ClusterRoleSelectors, metav1.LabelSelector{
		MatchLabels: map[string]string{
			openmcpv1alpha1.AdminNamespaceScopeMatchLabel: "true",
		},
	})

	if adminNamespaceScoped.Labels == nil {
		adminNamespaceScoped.Labels = make(map[string]string)
	}

	adminNamespaceScoped.Labels[openmcpv1alpha1.AdminNamespaceScopeMatchLabel] = "true"

	if adminClusterScoped.ClusterRoleSelectors == nil {
		adminClusterScoped.ClusterRoleSelectors = make([]metav1.LabelSelector, 0)
	}

	adminClusterScoped.ClusterRoleSelectors = append(adminClusterScoped.ClusterRoleSelectors, metav1.LabelSelector{
		MatchLabels: map[string]string{
			openmcpv1alpha1.AdminClusterScopeMatchLabel: "true",
		},
	})

	if adminClusterScoped.Labels == nil {
		adminClusterScoped.Labels = make(map[string]string)
	}

	adminClusterScoped.Labels[openmcpv1alpha1.AdminClusterScopeMatchLabel] = "true"

	// View Section
	view := &ac.View
	viewNamespaceScoped := &view.NamespaceScoped
	viewClusterScoped := &view.ClusterScoped

	if viewNamespaceScoped.ClusterRoleSelectors == nil {
		viewNamespaceScoped.ClusterRoleSelectors = make([]metav1.LabelSelector, 0)
	}

	viewNamespaceScoped.ClusterRoleSelectors = append(viewNamespaceScoped.ClusterRoleSelectors, metav1.LabelSelector{
		MatchLabels: map[string]string{
			openmcpv1alpha1.ViewNamespaceScopeMatchLabel: "true",
		},
	})

	if viewNamespaceScoped.Labels == nil {
		viewNamespaceScoped.Labels = make(map[string]string)
	}

	viewNamespaceScoped.Labels[openmcpv1alpha1.ViewNamespaceScopeMatchLabel] = "true"
	viewNamespaceScoped.Labels[openmcpv1alpha1.AdminNamespaceScopeMatchLabel] = "true"

	if viewClusterScoped.ClusterRoleSelectors == nil {
		viewClusterScoped.ClusterRoleSelectors = make([]metav1.LabelSelector, 0)
	}

	viewClusterScoped.ClusterRoleSelectors = append(viewClusterScoped.ClusterRoleSelectors, metav1.LabelSelector{
		MatchLabels: map[string]string{
			openmcpv1alpha1.ViewClusterScopeMatchLabel: "true",
		},
	})

	if viewClusterScoped.Labels == nil {
		viewClusterScoped.Labels = make(map[string]string)
	}

	viewClusterScoped.Labels[openmcpv1alpha1.ViewClusterScopeMatchLabel] = "true"
	viewClusterScoped.Labels[openmcpv1alpha1.AdminClusterScopeMatchLabel] = "true"

	if len(ac.ProtectedNamespaces) == 0 {
		ac.ProtectedNamespaces = []ProtectedNamespace{
			{
				Prefix: "kube-",
			},
			{
				Postfix: "-system",
			},
		}
	}

	// Set the compiled pattern for each protected namespace.
	for i := range ac.ProtectedNamespaces {
		pn := &ac.ProtectedNamespaces[i]

		if pn.Pattern != "" {
			pn.CompiledPattern = regexp.MustCompile(pn.Pattern)
		}
	}

	// Cluster Admin Section
	if ac.ClusterAdmin.ActiveDuration.Duration == 0 {
		ac.ClusterAdmin.ActiveDuration.Duration = 24 * time.Hour
	}
}

// GetRulesConfig returns the rules configuration for the given cluster role name.
func (ac *AuthorizationConfig) GetRulesConfig(clusterRoleName string) *RulesConfig {
	var rulesConfig *RulesConfig

	if openmcpv1alpha1.IsAdminRole(clusterRoleName) && openmcpv1alpha1.IsClusterScopedRole(clusterRoleName) {
		rulesConfig = &ac.Admin.ClusterScoped
	} else if openmcpv1alpha1.IsAdminRole(clusterRoleName) && !openmcpv1alpha1.IsClusterScopedRole(clusterRoleName) {
		rulesConfig = &ac.Admin.NamespaceScoped
	} else if !openmcpv1alpha1.IsAdminRole(clusterRoleName) && openmcpv1alpha1.IsClusterScopedRole(clusterRoleName) {
		rulesConfig = &ac.View.ClusterScoped
	} else if !openmcpv1alpha1.IsAdminRole(clusterRoleName) && !openmcpv1alpha1.IsClusterScopedRole(clusterRoleName) {
		rulesConfig = &ac.View.NamespaceScoped
	}

	return rulesConfig
}

// Validate validates the authorization configuration.
func Validate(config *AuthorizationConfig) error {
	allErrs := field.ErrorList{}

	path := field.NewPath("admin").Child("namespaceScoped").Child("rules")
	for i, rule := range config.Admin.NamespaceScoped.Rules {
		allErrs = append(allErrs, validateRule(rule, path.Index(i))...)
	}

	path = field.NewPath("admin").Child("clusterScoped").Child("rules")
	for i, rule := range config.Admin.ClusterScoped.Rules {
		allErrs = append(allErrs, validateRule(rule, path.Index(i))...)
	}

	path = field.NewPath("view").Child("namespaceScoped").Child("rules")
	for i, rule := range config.View.NamespaceScoped.Rules {
		allErrs = append(allErrs, validateRule(rule, path.Index(i))...)
	}

	path = field.NewPath("view").Child("clusterScoped").Child("rules")
	for i, rule := range config.View.ClusterScoped.Rules {
		allErrs = append(allErrs, validateRule(rule, path.Index(i))...)
	}

	path = field.NewPath("admin").Child("subjects")
	for i, subject := range config.Admin.AdditionalSubjects {
		allErrs = append(allErrs, validateSubject(&subject, path.Index(i))...)
	}

	path = field.NewPath("view").Child("subjects")
	for i, subject := range config.View.AdditionalSubjects {
		allErrs = append(allErrs, validateSubject(&subject, path.Index(i))...)
	}

	path = field.NewPath("protectedNamespaces")
	for i, pn := range config.ProtectedNamespaces {
		if pn.Pattern != "" {
			_, err := regexp.Compile(pn.Pattern)
			if err != nil {
				allErrs = append(allErrs, field.Invalid(path.Index(i).Child("pattern"), pn.Pattern, "pattern is invalid"))
			}
		}
	}

	return allErrs.ToAggregate()
}

// validateRule validates the given rule.
func validateRule(rule rbacv1.PolicyRule, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(rule.APIGroups) == 0 {
		allErrs = append(allErrs, field.Required(fieldPath.Child("apiGroups"), "apiGroups must be set"))
	}

	if len(rule.Resources) == 0 {
		allErrs = append(allErrs, field.Required(fieldPath.Child("resources"), "resources must be set"))
	}

	if len(rule.Verbs) == 0 {
		allErrs = append(allErrs, field.Required(fieldPath.Child("verbs"), "verbs must be set"))
	}

	return allErrs
}

func validateSubject(subject *rbacv1.Subject, fieldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if subject.Kind != rbacv1.UserKind && subject.Kind != rbacv1.GroupKind && subject.Kind != rbacv1.ServiceAccountKind {
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("kind"), subject.Kind, "kind must be either User, Group or ServiceAccount"))
	}

	if (subject.Kind == rbacv1.UserKind || subject.Kind == rbacv1.GroupKind) && subject.APIGroup != rbacv1.GroupName {
		allErrs = append(allErrs, field.Invalid(fieldPath.Child("apiGroup"), subject.APIGroup, "apiGroup must be set to "+rbacv1.GroupName))
	}

	if subject.Kind == rbacv1.ServiceAccountKind && subject.Namespace == "" {
		allErrs = append(allErrs, field.Required(fieldPath.Child("namespace"), "namespace must be set"))
	}

	if subject.Name == "" {
		allErrs = append(allErrs, field.Required(fieldPath.Child("name"), "name must be set"))
	}

	return allErrs
}

// IsAllowedNamespaceName returns true if the given namespace name is allowed to be modified by the user.
func (ac *AuthorizationConfig) IsAllowedNamespaceName(name string) bool {
	for _, pn := range ac.ProtectedNamespaces {
		if pn.Exact != "" && pn.Exact == name {
			return false
		}

		if pn.Prefix != "" && strings.HasPrefix(name, pn.Prefix) {
			return false
		}

		if pn.Postfix != "" && strings.HasSuffix(name, pn.Postfix) {
			return false
		}

		if pn.CompiledPattern != nil && pn.CompiledPattern.MatchString(name) {
			return false
		}
	}
	return true
}
