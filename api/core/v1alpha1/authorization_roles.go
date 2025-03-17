package v1alpha1

// GetClusterRoleNames returns the names of all known cluster roles.
func GetClusterRoleNames() []string {
	return []string{
		AdminNamespaceScopeRole,
		AdminClusterScopeRole,
		AdminNamespaceScopeStandardRulesRole,
		AdminClusterScopeStandardRulesRole,
		ViewNamespaceScopeRole,
		ViewClusterScopeRole,
		ViewNamespaceScopeStandardRulesRole,
		ViewClusterScopeStandardClusterRole,
	}
}

// IsAggregatedRole returns true if the given role name is an aggregated role.
func IsAggregatedRole(roleName string) bool {
	return roleName == AdminNamespaceScopeRole ||
		roleName == AdminClusterScopeRole ||
		roleName == ViewNamespaceScopeRole ||
		roleName == ViewClusterScopeRole
}

// IsAdminRole returns true if the given role name is an admin role.
func IsAdminRole(roleName string) bool {
	return roleName == AdminNamespaceScopeRole ||
		roleName == AdminClusterScopeRole ||
		roleName == AdminNamespaceScopeStandardRulesRole ||
		roleName == AdminClusterScopeStandardRulesRole
}

// IsClusterScopedRole returns true if the given role name is a cluster scoped role.
func IsClusterScopedRole(roleName string) bool {
	return roleName == AdminClusterScopeRole ||
		roleName == AdminClusterScopeStandardRulesRole ||
		roleName == ViewClusterScopeRole ||
		roleName == ViewClusterScopeStandardClusterRole
}
