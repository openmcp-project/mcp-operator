package config_test

import (
	"path"

	"github.com/openmcp-project/mcp-operator/internal/controller/core/authorization/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

var _ = Describe("Auth Config Utils", func() {
	It("should load the config from file", func() {
		authzConfig, err := config.LoadConfig(path.Join("testdata", "config_valid.yaml"))
		Expect(err).ToNot(HaveOccurred())
		Expect(authzConfig).ToNot(BeNil())

		admin := authzConfig.Admin
		adminNamespaceScoped := admin.NamespaceScoped
		adminClusterScoped := admin.ClusterScoped

		Expect(adminNamespaceScoped.ClusterRoleSelectors).To(HaveLen(1))
		Expect(adminNamespaceScoped.ClusterRoleSelectors[0].MatchLabels).To(HaveKeyWithValue(openmcpv1alpha1.AdminNamespaceScopeMatchLabel, "true"))
		Expect(adminNamespaceScoped.Labels).To(HaveKeyWithValue(openmcpv1alpha1.AdminNamespaceScopeMatchLabel, "true"))

		Expect(adminNamespaceScoped.Rules).To(HaveLen(1))
		Expect(adminNamespaceScoped.Rules).To(ConsistOf(rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"create", "update", "patch", "delete"},
		}))

		Expect(adminClusterScoped.ClusterRoleSelectors).To(HaveLen(0))
		Expect(adminClusterScoped.Labels).To(HaveLen(0))

		Expect(adminClusterScoped.Rules).To(HaveLen(1))
		Expect(adminClusterScoped.Rules).To(ConsistOf(rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"create", "update", "patch", "delete"},
		}))

		Expect(authzConfig.Admin.AdditionalSubjects).To(HaveLen(2))
		Expect(authzConfig.Admin.AdditionalSubjects).To(ConsistOf(
			rbacv1.Subject{Kind: "User", Name: "system-admin", APIGroup: rbacv1.GroupName},
			rbacv1.Subject{Kind: "Group", Name: "system:admins", APIGroup: rbacv1.GroupName},
		))

		view := authzConfig.View
		viewNamespaceScoped := view.NamespaceScoped
		viewClusterScoped := view.ClusterScoped

		Expect(viewNamespaceScoped.ClusterRoleSelectors).To(HaveLen(1))
		Expect(viewNamespaceScoped.ClusterRoleSelectors[0].MatchLabels).To(HaveKeyWithValue(openmcpv1alpha1.ViewNamespaceScopeMatchLabel, "true"))
		Expect(viewNamespaceScoped.Labels).To(HaveKeyWithValue(openmcpv1alpha1.ViewNamespaceScopeMatchLabel, "true"))

		Expect(viewNamespaceScoped.Rules).To(HaveLen(1))
		Expect(viewNamespaceScoped.Rules).To(ConsistOf(rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list", "watch"},
		}))

		Expect(viewClusterScoped.ClusterRoleSelectors).To(HaveLen(0))
		Expect(viewClusterScoped.Labels).To(HaveLen(0))

		Expect(viewClusterScoped.Rules).To(HaveLen(1))
		Expect(viewClusterScoped.Rules).To(ConsistOf(rbacv1.PolicyRule{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "list", "watch"},
		}))

		Expect(authzConfig.View.AdditionalSubjects).To(ConsistOf(
			rbacv1.Subject{Kind: "ServiceAccount", Name: "manager", Namespace: "openmcp-system", APIGroup: ""},
		))
	})

	It("should fail to load the config from file", func() {
		authzConfig, err := config.LoadConfig(path.Join("testdata", "config_invalid.yaml"))
		Expect(err).To(HaveOccurred())
		Expect(authzConfig).To(BeNil())

		authzConfig, err = config.LoadConfig(path.Join("testdata", "config_missing.yaml"))
		Expect(err).To(HaveOccurred())
		Expect(authzConfig).To(BeNil())
	})
})
