package config_test

import (
	"errors"

	authzconfig "github.tools.sap/CoLa/mcp-operator/internal/controller/core/authorization/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

var _ = Describe("Authorization Config", func() {
	It("should set defaults", func() {
		config := &authzconfig.AuthorizationConfig{}
		config.SetDefaults()

		Expect(config.Admin.NamespaceScoped.Labels).To(HaveKeyWithValue(openmcpv1alpha1.AdminNamespaceScopeMatchLabel, "true"))
		Expect(config.Admin.NamespaceScoped.ClusterRoleSelectors).To(HaveLen(1))
		Expect(config.Admin.NamespaceScoped.ClusterRoleSelectors[0].MatchLabels).To(HaveKeyWithValue(openmcpv1alpha1.AdminNamespaceScopeMatchLabel, "true"))

		Expect(config.Admin.ClusterScoped.Labels).To(HaveKeyWithValue(openmcpv1alpha1.AdminClusterScopeMatchLabel, "true"))
		Expect(config.Admin.ClusterScoped.ClusterRoleSelectors).To(HaveLen(1))
		Expect(config.Admin.ClusterScoped.ClusterRoleSelectors[0].MatchLabels).To(HaveKeyWithValue(openmcpv1alpha1.AdminClusterScopeMatchLabel, "true"))

		Expect(config.View.NamespaceScoped.Labels).To(HaveKeyWithValue(openmcpv1alpha1.ViewNamespaceScopeMatchLabel, "true"))
		Expect(config.View.NamespaceScoped.ClusterRoleSelectors).To(HaveLen(1))
		Expect(config.View.NamespaceScoped.ClusterRoleSelectors[0].MatchLabels).To(HaveKeyWithValue(openmcpv1alpha1.ViewNamespaceScopeMatchLabel, "true"))

		Expect(config.View.ClusterScoped.Labels).To(HaveKeyWithValue(openmcpv1alpha1.ViewClusterScopeMatchLabel, "true"))
		Expect(config.View.ClusterScoped.ClusterRoleSelectors).To(HaveLen(1))
		Expect(config.View.ClusterScoped.ClusterRoleSelectors[0].MatchLabels).To(HaveKeyWithValue(openmcpv1alpha1.ViewClusterScopeMatchLabel, "true"))
	})

	It("should not validate", func() {
		config := &authzconfig.AuthorizationConfig{}

		config.Admin.NamespaceScoped.Rules = []rbacv1.PolicyRule{
			{}, // 3 errors
		}
		config.Admin.ClusterScoped.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""}, // 2 errors
			},
		}
		config.View.NamespaceScoped.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"pods"}, // 1 error
			},
		}
		config.View.ClusterScoped.Rules = []rbacv1.PolicyRule{
			{
				Verbs: []string{"get"}, // 2 errors
			},
		}

		config.Admin.AdditionalSubjects = []rbacv1.Subject{
			{
				Kind: "User", // 2 errors
			},
			{
				Kind:     "Group", // 1 errors
				APIGroup: "invalid",
				Name:     "foo",
			},
			{
				Kind: "Unknown", // 1 error
				Name: "foo",
			},
		}

		config.View.AdditionalSubjects = []rbacv1.Subject{
			{
				Kind: "ServiceAccount", // 2 errors
			},
		}

		err := authzconfig.Validate(config)
		Expect(err).To(HaveOccurred())
		var errorList utilerrors.Aggregate
		ok := errors.As(err, &errorList)
		Expect(ok).To(BeTrue())
		Expect(errorList.Errors()).To(HaveLen(14))

	})

	Context("RulesConfig", func() {
		It("returns Admin NamespaceScoped RulesConfig for admin namespace scoped role", func() {
			config := &authzconfig.AuthorizationConfig{}
			config.Admin.NamespaceScoped = authzconfig.RulesConfig{}
			rulesConfig := config.GetRulesConfig(openmcpv1alpha1.AdminNamespaceScopeRole)
			Expect(rulesConfig).To(Equal(&config.Admin.NamespaceScoped))
		})

		It("returns Admin ClusterScoped RulesConfig for admin cluster scoped role", func() {
			config := &authzconfig.AuthorizationConfig{}
			config.Admin.ClusterScoped = authzconfig.RulesConfig{}
			rulesConfig := config.GetRulesConfig(openmcpv1alpha1.AdminClusterScopeRole)
			Expect(rulesConfig).To(Equal(&config.Admin.ClusterScoped))
		})

		It("returns View NamespaceScoped RulesConfig for view namespace scoped role", func() {
			config := &authzconfig.AuthorizationConfig{}
			config.View.NamespaceScoped = authzconfig.RulesConfig{}
			rulesConfig := config.GetRulesConfig(openmcpv1alpha1.ViewNamespaceScopeRole)
			Expect(rulesConfig).To(Equal(&config.View.NamespaceScoped))
		})

		It("returns View ClusterScoped RulesConfig for view cluster scoped role", func() {
			config := &authzconfig.AuthorizationConfig{}
			config.View.ClusterScoped = authzconfig.RulesConfig{}
			rulesConfig := config.GetRulesConfig(openmcpv1alpha1.ViewClusterScopeRole)
			Expect(rulesConfig).To(Equal(&config.View.ClusterScoped))
		})
	})
})

var _ = Describe("IsAllowedNamespaceName", func() {
	Context("defaults", func() {
		It("returns true for valid namespace names", func() {
			config := &authzconfig.AuthorizationConfig{}
			config.SetDefaults()
			Expect(config.IsAllowedNamespaceName("valid-namespace")).To(BeTrue())
			Expect(config.IsAllowedNamespaceName("another-valid-namespace")).To(BeTrue())
		})

		It("returns false for namespace names starting with 'kube-'", func() {
			config := &authzconfig.AuthorizationConfig{}
			config.SetDefaults()
			Expect(config.IsAllowedNamespaceName("kube-system")).To(BeFalse())
			Expect(config.IsAllowedNamespaceName("kube-public")).To(BeFalse())
		})

		It("returns false for namespace names ending with '-system'", func() {
			config := &authzconfig.AuthorizationConfig{}
			config.SetDefaults()
			Expect(config.IsAllowedNamespaceName("default-system")).To(BeFalse())
			Expect(config.IsAllowedNamespaceName("my-system")).To(BeFalse())
			Expect(config.IsAllowedNamespaceName("my-system")).To(BeFalse())
		})
	})

	Context("custom", func() {
		var (
			config *authzconfig.AuthorizationConfig
		)

		BeforeEach(func() {
			config = &authzconfig.AuthorizationConfig{
				ProtectedNamespaces: []authzconfig.ProtectedNamespace{
					{
						Exact: "foobar",
					},
					{
						Prefix:  "protected-",
						Postfix: "-protected",
					},
					{
						Pattern: "custom-.*-pattern",
					},
				},
			}
			config.SetDefaults()
			Expect(authzconfig.Validate(config)).To(Succeed())
		})

		It("returns true for valid namespace names", func() {
			Expect(config.IsAllowedNamespaceName("valid-namespace")).To(BeTrue())
			Expect(config.IsAllowedNamespaceName("another-valid-namespace")).To(BeTrue())
		})

		It("returns false for exact match namespace names", func() {
			Expect(config.IsAllowedNamespaceName("foobar")).To(BeFalse())
		})

		It("returns false for namespace names starting with 'protected-'", func() {
			Expect(config.IsAllowedNamespaceName("protected-system")).To(BeFalse())
			Expect(config.IsAllowedNamespaceName("protected-public")).To(BeFalse())
		})

		It("returns false for namespace names ending with '-protected'", func() {
			Expect(config.IsAllowedNamespaceName("default-protected")).To(BeFalse())
			Expect(config.IsAllowedNamespaceName("my-protected")).To(BeFalse())
		})

		It("returns false for namespace names matching 'custom-.*-pattern'", func() {
			Expect(config.IsAllowedNamespaceName("custom-foo-pattern")).To(BeFalse())
			Expect(config.IsAllowedNamespaceName("custom-bar-pattern")).To(BeFalse())
		})

		It("should not validate an invalid pattern", func() {
			config.ProtectedNamespaces = append(config.ProtectedNamespaces, authzconfig.ProtectedNamespace{
				Pattern: "[",
			})
			Expect(authzconfig.Validate(config)).To(HaveOccurred())
		})
	})
})
