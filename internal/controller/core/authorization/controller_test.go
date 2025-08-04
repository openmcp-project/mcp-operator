package authorization_test

import (
	"strings"

	components "github.com/openmcp-project/mcp-operator/internal/components"

	"github.com/openmcp-project/mcp-operator/internal/controller/core/authorization"
	"github.com/openmcp-project/mcp-operator/internal/controller/core/authorization/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/openmcp-project/mcp-operator/test/matchers"

	"github.com/openmcp-project/controller-utils/pkg/testing"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	testutils "github.com/openmcp-project/mcp-operator/test/utils"
)

const (
	adminVerbs = "create,update,patch,delete"
	viewVerbs  = "get,list,watch"

	namespaceScopedResource = "secrets"
	clusterScopedResource   = "namespaces"

	aggregateToAdminLabel = "rbac.dummy.local/aggregate-to-admin"
	aggregateToViewLabel  = "rbac.dummy.local/aggregate-to-view"

	aggregateToAdminClusterScopedLabel = "rbac.dummy.local/aggregate-to-admin-clusterscoped"
	aggregateToViewClusterScopedLabel  = "rbac.dummy.local/aggregate-to-view-clusterscoped"

	authzReconciler = "authz"
)

func getReconciler(c ...client.Client) reconcile.Reconciler {
	return authorization.NewAuthorizationReconciler(c[0], &config.AuthorizationConfig{
		Admin: config.RoleConfig{
			AdditionalSubjects: []rbacv1.Subject{
				{
					Kind:     rbacv1.UserKind,
					Name:     "static-admin",
					APIGroup: rbacv1.GroupName,
				},
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      "static-manager",
					Namespace: "openmcp-system",
				},
			},
			NamespaceScoped: config.RulesConfig{
				Labels: map[string]string{
					aggregateToAdminLabel: "true",
				},
				ClusterRoleSelectors: []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{
							aggregateToAdminLabel: "true",
						},
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{namespaceScopedResource},
						Verbs:     strings.Split(adminVerbs, ","),
					},
				},
			},
			ClusterScoped: config.RulesConfig{
				Labels: map[string]string{
					aggregateToAdminClusterScopedLabel: "true",
				},
				ClusterRoleSelectors: []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{
							aggregateToAdminClusterScopedLabel: "true",
						},
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{clusterScopedResource},
						Verbs:     strings.Split(adminVerbs, ","),
					},
				},
			},
		},
		View: config.RoleConfig{
			AdditionalSubjects: []rbacv1.Subject{
				{
					Kind:     rbacv1.GroupKind,
					Name:     "static-auditors",
					APIGroup: rbacv1.GroupName,
				},
			},
			NamespaceScoped: config.RulesConfig{
				Labels: map[string]string{
					aggregateToViewLabel: "true",
				},
				ClusterRoleSelectors: []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{
							aggregateToViewLabel: "true",
						},
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{namespaceScopedResource},
						Verbs:     strings.Split(viewVerbs, ","),
					},
				},
			},
			ClusterScoped: config.RulesConfig{
				Labels: map[string]string{
					aggregateToViewClusterScopedLabel: "true",
				},
				ClusterRoleSelectors: []metav1.LabelSelector{
					{
						MatchLabels: map[string]string{
							aggregateToViewClusterScopedLabel: "true",
						},
					},
				},
				Rules: []rbacv1.PolicyRule{
					{
						APIGroups: []string{""},
						Resources: []string{clusterScopedResource},
						Verbs:     strings.Split(viewVerbs, ","),
					},
				},
			},
		},
	})
}

func testEnvWithAPIServerAccess(testDataPathSegments ...string) *testing.ComplexEnvironment {
	env := testutils.DefaultTestSetupBuilder(testDataPathSegments...).WithFakeClient(testutils.APIServerCluster, testutils.Scheme).WithReconcilerConstructor(authzReconciler, getReconciler, testutils.CrateCluster).Build()
	env.Reconcilers[authzReconciler].(*authorization.AuthorizationReconciler).SetAPIServerAccess(&testutils.TestAPIServerAccess{Client: env.Client(testutils.APIServerCluster)})
	return env
}

var _ = Describe("CO-1153 Authorization Controller", func() {
	It("should set the status condition to false when there is no APIServer available", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-01")

		authz := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(authz)
		res := env.ShouldReconcile(authzReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)
		Expect(err).ToNot(HaveOccurred())

		Expect(authz.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthorizationComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthorizationComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForDependencies,
			}),
		))
	})

	It("should set the ready condition to false when APIServer is not ready", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-02")

		authz := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(authz)
		res := env.ShouldReconcile(authzReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)
		Expect(err).ToNot(HaveOccurred())

		Expect(authz.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthorizationComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthorizationComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForDependencies,
			}),
		))
	})

	It("should fail to reconcile and set the status condition to false when APIServer status has no access kubeconfig", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-03")

		authz := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(authz)
		_ = env.ShouldNotReconcile(authzReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)
		Expect(err).ToNot(HaveOccurred())

		Expect(authz.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthorizationComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthorizationComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonReconciliationError,
			}),
		))
	})

	It("should set the finalizers", func() {
		var err error
		env := testEnvWithAPIServerAccess("testdata", "test-04")

		authz := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)
		Expect(err).ToNot(HaveOccurred())
		Expect(authz.Finalizers).To(ContainElement(openmcpv1alpha1.AuthorizationComponent.Finalizer()))

		as := &openmcpv1alpha1.APIServer{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		authzComp := components.Component(authz)
		Expect(as.Finalizers).To(ContainElements(authzComp.Type().DependencyFinalizer()))
	})

	Context("admin cluster roles/bindings", func() {
		var err error
		var env *testing.ComplexEnvironment

		BeforeEach(func() {
			env = testEnvWithAPIServerAccess("testdata", "test-04")
		})

		It("should create cluster roles", func() {
			authz := &openmcpv1alpha1.Authorization{}
			err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
			Expect(err).ToNot(HaveOccurred())

			req := testing.RequestFromObject(authz)
			_ = env.ShouldReconcile(authzReconciler, req)

			err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)
			Expect(err).ToNot(HaveOccurred())
			Expect(authz.Status.Conditions).To(ContainElements(
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   openmcpv1alpha1.AuthorizationComponent.ReconciliationCondition(),
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
			))

			adminNamespaceScopeClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: openmcpv1alpha1.AdminNamespaceScopeRole,
				},
			}
			err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(adminNamespaceScopeClusterRole), adminNamespaceScopeClusterRole)
			Expect(err).ToNot(HaveOccurred())
			verifyAggregationClusterRole(adminNamespaceScopeClusterRole)

			adminClusterScopeClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: openmcpv1alpha1.AdminClusterScopeRole,
				},
			}
			err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(adminClusterScopeClusterRole), adminClusterScopeClusterRole)
			Expect(err).ToNot(HaveOccurred())
			verifyAggregationClusterRole(adminClusterScopeClusterRole)

			adminNamespaceScopeClusterRoleBinding := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: openmcpv1alpha1.AdminNamespaceScopeStandardRulesRole,
				},
			}
			err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(adminNamespaceScopeClusterRoleBinding), adminNamespaceScopeClusterRoleBinding)
			Expect(err).ToNot(HaveOccurred())
			verifyStandardClusterRole(adminNamespaceScopeClusterRoleBinding)

			adminClusterScopeClusterRoleBinding := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: openmcpv1alpha1.AdminClusterScopeStandardRulesRole,
				},
			}
			err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(adminClusterScopeClusterRoleBinding), adminClusterScopeClusterRoleBinding)
			Expect(err).ToNot(HaveOccurred())
			verifyStandardClusterRole(adminClusterScopeClusterRoleBinding)
		})

		It("should create cluster role bindings", func() {
			authz := &openmcpv1alpha1.Authorization{}
			err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
			Expect(err).ToNot(HaveOccurred())

			// set the defaults for the API groups
			authz.Spec.Default()
			err = env.Client(testutils.CrateCluster).Update(env.Ctx, authz)
			Expect(err).ToNot(HaveOccurred())

			req := testing.RequestFromObject(authz)
			_ = env.ShouldReconcile(authzReconciler, req)

			err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)
			Expect(err).ToNot(HaveOccurred())
			Expect(authz.Status.Conditions).To(ContainElements(
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   openmcpv1alpha1.AuthorizationComponent.ReconciliationCondition(),
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
			))

			adminClusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: openmcpv1alpha1.AdminClusterRoleBinding,
				},
			}
			err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(adminClusterRoleBinding), adminClusterRoleBinding)
			Expect(err).ToNot(HaveOccurred())
			Expect(adminClusterRoleBinding.RoleRef.Name).To(Equal(openmcpv1alpha1.AdminClusterScopeRole))
			Expect(adminClusterRoleBinding.Subjects).To(ConsistOf(
				rbacv1.Subject{
					Kind:     rbacv1.UserKind,
					Name:     "admin",
					APIGroup: rbacv1.GroupName,
				},
				rbacv1.Subject{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      "pipeline",
					Namespace: "automate",
				},
				rbacv1.Subject{
					Kind:     rbacv1.UserKind,
					Name:     "static-admin",
					APIGroup: rbacv1.GroupName,
				},
				rbacv1.Subject{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      "static-manager",
					Namespace: "openmcp-system",
				}))
		})
	})

	Context("view cluster roles/bindings", func() {
		var err error
		var env *testing.ComplexEnvironment

		BeforeEach(func() {
			env = testEnvWithAPIServerAccess("testdata", "test-04")
		})

		It("should create cluster roles", func() {
			authz := &openmcpv1alpha1.Authorization{}
			err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
			Expect(err).ToNot(HaveOccurred())

			req := testing.RequestFromObject(authz)
			_ = env.ShouldReconcile(authzReconciler, req)

			err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)
			Expect(err).ToNot(HaveOccurred())
			Expect(authz.Status.Conditions).To(ContainElements(
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   openmcpv1alpha1.AuthorizationComponent.ReconciliationCondition(),
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
			))

			viewNamespaceScopeClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: openmcpv1alpha1.ViewNamespaceScopeRole,
				},
			}
			err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(viewNamespaceScopeClusterRole), viewNamespaceScopeClusterRole)
			Expect(err).ToNot(HaveOccurred())
			verifyAggregationClusterRole(viewNamespaceScopeClusterRole)

			viewClusterScopeClusterRole := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: openmcpv1alpha1.ViewClusterScopeRole,
				},
			}
			err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(viewClusterScopeClusterRole), viewClusterScopeClusterRole)
			Expect(err).ToNot(HaveOccurred())
			verifyAggregationClusterRole(viewClusterScopeClusterRole)

			viewNamespaceScopeClusterRoleBinding := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: openmcpv1alpha1.ViewNamespaceScopeStandardRulesRole,
				},
			}
			err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(viewNamespaceScopeClusterRoleBinding), viewNamespaceScopeClusterRoleBinding)
			Expect(err).ToNot(HaveOccurred())
			verifyStandardClusterRole(viewNamespaceScopeClusterRoleBinding)

			viewClusterScopeClusterRoleBinding := &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: openmcpv1alpha1.ViewClusterScopeStandardClusterRole,
				},
			}
			err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(viewClusterScopeClusterRoleBinding), viewClusterScopeClusterRoleBinding)
			Expect(err).ToNot(HaveOccurred())
			verifyStandardClusterRole(viewClusterScopeClusterRoleBinding)
		})

		It("should create cluster role bindings", func() {
			authz := &openmcpv1alpha1.Authorization{}
			err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
			Expect(err).ToNot(HaveOccurred())

			// set the defaults for the API groups
			authz.Spec.Default()
			err = env.Client(testutils.CrateCluster).Update(env.Ctx, authz)
			Expect(err).ToNot(HaveOccurred())

			req := testing.RequestFromObject(authz)
			_ = env.ShouldReconcile(authzReconciler, req)

			err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)
			Expect(err).ToNot(HaveOccurred())
			Expect(authz.Finalizers).To(ContainElement(openmcpv1alpha1.AuthorizationComponent.Finalizer()))
			Expect(authz.Status.Conditions).To(ContainElements(
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   openmcpv1alpha1.AuthorizationComponent.ReconciliationCondition(),
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
			))

			viewClusterRoleBinding := &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: openmcpv1alpha1.ViewClusterRoleBinding,
				},
			}
			err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(viewClusterRoleBinding), viewClusterRoleBinding)
			Expect(err).ToNot(HaveOccurred())
			Expect(viewClusterRoleBinding.RoleRef.Name).To(Equal(openmcpv1alpha1.ViewClusterScopeRole))
			Expect(viewClusterRoleBinding.Subjects).To(ConsistOf(
				rbacv1.Subject{
					Kind:     rbacv1.GroupKind,
					Name:     "auditors",
					APIGroup: rbacv1.GroupName,
				},
				rbacv1.Subject{
					Kind:     rbacv1.GroupKind,
					Name:     "static-auditors",
					APIGroup: rbacv1.GroupName,
				}))
		})
	})

	It("should add namespace resource name in cluster role for user namespace", func() {
		var err error
		env := testEnvWithAPIServerAccess("testdata", "test-04")

		// create user namespace
		namespace := v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "user",
			},
		}
		err = env.Client(testutils.APIServerCluster).Create(env.Ctx, &namespace)
		Expect(err).ToNot(HaveOccurred())

		authz := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		testWorker := testutils.NewTestWorker(env.Client(testutils.CrateCluster), env.Client(testutils.APIServerCluster))
		controller := env.Reconciler(authzReconciler).(*authorization.AuthorizationReconciler)
		controller.RegisterTasks(testWorker)

		as := &openmcpv1alpha1.APIServer{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).ToNot(HaveOccurred())

		// run tasks to update the user namespaces in the authorization status
		// it then should contain the user namespace name and the reconcile annotation should be set
		err = testWorker.RunTasks(env.Ctx, as)
		Expect(err).ToNot(HaveOccurred())

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())
		Expect(authz.Annotations).To(HaveKeyWithValue(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueReconcile))
		Expect(authz.Status.UserNamespaces).To(ConsistOf(namespace.Name))

		req = testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		// the user namespace should be added to the cluster role
		adminClusterScopeRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: openmcpv1alpha1.AdminClusterScopeStandardRulesRole,
			},
		}
		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(adminClusterScopeRole), adminClusterScopeRole)
		Expect(err).ToNot(HaveOccurred())
		Expect(adminClusterScopeRole.Rules).To(HaveLen(2))
		Expect(adminClusterScopeRole.Rules).To(ConsistOf([]rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{clusterScopedResource},
				Verbs:     strings.Split(adminVerbs, ","),
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"update", "patch", "delete"},
				ResourceNames: []string{
					namespace.Name,
				},
			},
		}))

		// delete the user namespace
		err = env.Client(testutils.APIServerCluster).Delete(env.Ctx, &namespace)
		Expect(err).ToNot(HaveOccurred())

		// run tasks to update the user namespaces in the authorization status
		// it then should not contain the user namespace name and the reconcile annotation should be set
		err = testWorker.RunTasks(env.Ctx, as)
		Expect(err).ToNot(HaveOccurred())

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())
		Expect(authz.Annotations).To(HaveKeyWithValue(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueReconcile))
		Expect(authz.Status.UserNamespaces).To(BeEmpty())

		req = testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		// the user namespace should be removed from the cluster role
		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(adminClusterScopeRole), adminClusterScopeRole)
		Expect(err).ToNot(HaveOccurred())
		Expect(adminClusterScopeRole.Rules).To(HaveLen(1))
		Expect(adminClusterScopeRole.Rules).To(ConsistOf([]rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{clusterScopedResource},
				Verbs:     strings.Split(adminVerbs, ","),
			},
		}))
	})

	It("should not add a namespace which is not allowed", func() {
		var err error
		env := testEnvWithAPIServerAccess("testdata", "test-04")

		// create user namespace
		namespace := v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-system",
			},
		}
		err = env.Client(testutils.APIServerCluster).Create(env.Ctx, &namespace)
		Expect(err).ToNot(HaveOccurred())

		authz := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		testWorker := testutils.NewTestWorker(env.Client(testutils.CrateCluster), env.Client(testutils.APIServerCluster))
		controller := env.Reconciler(authzReconciler).(*authorization.AuthorizationReconciler)
		controller.RegisterTasks(testWorker)

		as := &openmcpv1alpha1.APIServer{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).ToNot(HaveOccurred())

		// run tasks to update the user namespaces in the authorization status
		// it then should not contain the system namespace name and the reconcile annotation should not be set
		err = testWorker.RunTasks(env.Ctx, as)
		Expect(err).ToNot(HaveOccurred())

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())
		Expect(authz.Annotations).ToNot(HaveKeyWithValue(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueReconcile))
		Expect(authz.Status.UserNamespaces).To(BeEmpty())

		req = testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		// the user namespace should be added to the cluster role
		adminClusterScopeRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: openmcpv1alpha1.AdminClusterScopeStandardRulesRole,
			},
		}
		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(adminClusterScopeRole), adminClusterScopeRole)
		Expect(err).ToNot(HaveOccurred())
		Expect(adminClusterScopeRole.Rules).To(HaveLen(1))
		Expect(adminClusterScopeRole.Rules).To(ConsistOf([]rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{clusterScopedResource},
				Verbs:     strings.Split(adminVerbs, ","),
			},
		}))
	})

	It("should create namespaced role bindings", func() {
		var err error
		env := testEnvWithAPIServerAccess("testdata", "test-04")

		// create user namespace
		namespace := v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "user",
			},
		}
		err = env.Client(testutils.APIServerCluster).Create(env.Ctx, &namespace)
		Expect(err).ToNot(HaveOccurred())

		authz := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		// set the defaults for the API groups
		authz.Spec.Default()
		err = env.Client(testutils.CrateCluster).Update(env.Ctx, authz)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		testWorker := testutils.NewTestWorker(env.Client(testutils.CrateCluster), env.Client(testutils.APIServerCluster))
		controller := env.Reconciler(authzReconciler).(*authorization.AuthorizationReconciler)
		controller.RegisterTasks(testWorker)

		as := &openmcpv1alpha1.APIServer{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).ToNot(HaveOccurred())

		// run tasks to update the user namespaces in the authorization status
		// it then should not contain the system namespace name
		err = testWorker.RunTasks(env.Ctx, as)
		Expect(err).ToNot(HaveOccurred())

		req = testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		adminRoleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      openmcpv1alpha1.AdminRoleBinding,
				Namespace: namespace.Name,
			},
		}
		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(adminRoleBinding), adminRoleBinding)
		Expect(err).ToNot(HaveOccurred())
		Expect(adminRoleBinding.RoleRef.Name).To(Equal(openmcpv1alpha1.AdminNamespaceScopeRole))
		Expect(adminRoleBinding.Subjects).To(HaveLen(4))
		Expect(adminRoleBinding.Subjects).To(ConsistOf([]rbacv1.Subject{
			{
				Kind:     rbacv1.UserKind,
				Name:     "admin",
				APIGroup: rbacv1.GroupName,
			},
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "pipeline",
				Namespace: "automate",
			},
			{
				Kind:     rbacv1.UserKind,
				Name:     "static-admin",
				APIGroup: rbacv1.GroupName,
			},
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "static-manager",
				Namespace: "openmcp-system",
			},
		},
		))

		viewRoleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      openmcpv1alpha1.ViewRoleBinding,
				Namespace: namespace.Name,
			},
		}
		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(viewRoleBinding), viewRoleBinding)
		Expect(err).ToNot(HaveOccurred())
		Expect(viewRoleBinding.RoleRef.Name).To(Equal(openmcpv1alpha1.ViewNamespaceScopeRole))
		Expect(viewRoleBinding.Subjects).To(HaveLen(2))
		Expect(viewRoleBinding.Subjects).To(ConsistOf([]rbacv1.Subject{
			{
				Kind:     rbacv1.GroupKind,
				Name:     "auditors",
				APIGroup: rbacv1.GroupName,
			},
			{
				Kind:     rbacv1.GroupKind,
				Name:     "static-auditors",
				APIGroup: rbacv1.GroupName,
			},
		},
		))
	})

	It("should not create namespaced role bindings for system namespaces", func() {
		var err error
		env := testEnvWithAPIServerAccess("testdata", "test-04")

		// create user namespace
		namespace := v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-system",
			},
		}
		err = env.Client(testutils.APIServerCluster).Create(env.Ctx, &namespace)
		Expect(err).ToNot(HaveOccurred())

		authz := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		testWorker := testutils.NewTestWorker(env.Client(testutils.CrateCluster), env.Client(testutils.APIServerCluster))
		controller := env.Reconciler(authzReconciler).(*authorization.AuthorizationReconciler)
		controller.RegisterTasks(testWorker)

		as := &openmcpv1alpha1.APIServer{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).ToNot(HaveOccurred())

		// run tasks to update the user namespaces in the authorization status
		// it then should not contain the system namespace name
		err = testWorker.RunTasks(env.Ctx, as)
		Expect(err).ToNot(HaveOccurred())

		req = testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		roleBindings := &rbacv1.RoleBindingList{}
		err = env.Client(testutils.APIServerCluster).List(env.Ctx, roleBindings, client.InNamespace(namespace.Name))
		Expect(err).ToNot(HaveOccurred())
		Expect(roleBindings.Items).To(BeEmpty())
	})

	It("should handle delete of the authorization object", func() {
		var err error
		env := testEnvWithAPIServerAccess("testdata", "test-05")

		authz := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, authz)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())

		clusterRoles := &rbacv1.ClusterRoleList{}
		err = env.Client(testutils.APIServerCluster).List(env.Ctx, clusterRoles)
		Expect(err).ToNot(HaveOccurred())
		Expect(clusterRoles.Items).To(BeEmpty())

		clusterRoleBindings := &rbacv1.ClusterRoleBindingList{}
		err = env.Client(testutils.APIServerCluster).List(env.Ctx, clusterRoleBindings)
		Expect(err).ToNot(HaveOccurred())
		Expect(clusterRoleBindings.Items).To(BeEmpty())

		roleBindings := &rbacv1.RoleBindingList{}
		err = env.Client(testutils.APIServerCluster).List(env.Ctx, roleBindings)
		Expect(err).ToNot(HaveOccurred())
		Expect(roleBindings.Items).To(BeEmpty())

		as := &openmcpv1alpha1.APIServer{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).ToNot(HaveOccurred())
		Expect(as.Finalizers).To(BeEmpty())
	})

	It("reconcile should handle when authentication is not found", func() {
		env := testutils.DefaultTestSetupBuilder().WithReconcilerConstructor(authzReconciler, getReconciler, testutils.CrateCluster).Build()

		env.ShouldReconcile(authzReconciler, testing.RequestFromObject(&openmcpv1alpha1.Authorization{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
		}))
	})

	It("should handle the ignore annotation", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-06")

		auth := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(auth)
		_ = env.ShouldReconcile(authzReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())
		Expect(auth.Status.Conditions).To(HaveLen(0))

		clusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: openmcpv1alpha1.AdminNamespaceScopeRole,
			},
		}

		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(clusterRole), clusterRole)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("should not be deleted when it has a dependency finalizer", func() {
		var err error

		env := testEnvWithAPIServerAccess("testdata", "test-07")

		auth := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)
		Expect(err).NotTo(HaveOccurred())

		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, auth)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(auth)
		_ = env.ShouldReconcile(authzReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)
		Expect(err).NotTo(HaveOccurred())
		Expect(auth.Finalizers).To(ContainElement(openmcpv1alpha1.AuthorizationComponent.Finalizer()))
		Expect(auth.Finalizers).To(ContainElement("dependency." + openmcpv1alpha1.BaseDomain + "/other_comp"))
	})

	It("should not handle namespaces with deletion timestamp", func() {
		var err error
		env := testEnvWithAPIServerAccess("testdata", "test-04")

		// 1. create user namespace
		// 2. delete user namespace
		// 3. run tasks to update the user namespaces in the authorization status
		// 4. run reconcile
		// 5. verify that the user namespace is not added to the authorization status
		// 6. verify that the role bindings are not being created
		namespace := v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "user-deleting",
				Finalizers: []string{"openmcp.cloud/testing"},
			},
		}
		err = env.Client(testutils.APIServerCluster).Create(env.Ctx, &namespace)
		Expect(err).ToNot(HaveOccurred())

		err = env.Client(testutils.APIServerCluster).Delete(env.Ctx, &namespace)
		Expect(err).ToNot(HaveOccurred())

		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(&namespace), &namespace)
		Expect(err).ToNot(HaveOccurred())

		authz := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		testWorker := testutils.NewTestWorker(env.Client(testutils.CrateCluster), env.Client(testutils.APIServerCluster))
		controller := env.Reconciler(authzReconciler).(*authorization.AuthorizationReconciler)
		controller.RegisterTasks(testWorker)

		as := &openmcpv1alpha1.APIServer{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).ToNot(HaveOccurred())

		// run tasks to update the user namespaces in the authorization status
		// it then should not contain the namespace being deleted
		err = testWorker.RunTasks(env.Ctx, as)
		Expect(err).ToNot(HaveOccurred())

		req = testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)
		Expect(err).ToNot(HaveOccurred())
		Expect(authz.Status.UserNamespaces).To(BeEmpty())

		roleBindings := &rbacv1.RoleBindingList{}
		err = env.Client(testutils.APIServerCluster).List(env.Ctx, roleBindings, client.InNamespace(namespace.Name))
		Expect(err).ToNot(HaveOccurred())
		Expect(roleBindings.Items).To(BeEmpty())

		// 1. create user namespace
		// 2. run tasks to update the user namespaces in the authorization status
		// 3. delete user namespace
		// 4. run reconcile
		// 5. verify that the user namespace is added to the authorization status
		// 6. verify that the role bindings are not being created
		namespace = v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "user-deleting2",
				Finalizers: []string{"openmcp.cloud/testing"},
			},
		}
		err = env.Client(testutils.APIServerCluster).Create(env.Ctx, &namespace)
		Expect(err).ToNot(HaveOccurred())

		// run tasks to update the user namespaces in the authorization status
		// it then should contain the namespace
		err = testWorker.RunTasks(env.Ctx, as)
		Expect(err).ToNot(HaveOccurred())

		err = env.Client(testutils.APIServerCluster).Delete(env.Ctx, &namespace)
		Expect(err).ToNot(HaveOccurred())

		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(&namespace), &namespace)
		Expect(err).ToNot(HaveOccurred())

		req = testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)
		Expect(err).ToNot(HaveOccurred())
		Expect(authz.Status.UserNamespaces).ToNot(BeEmpty())
		Expect(authz.Status.UserNamespaces).To(ConsistOf(namespace.Name))

		roleBindings = &rbacv1.RoleBindingList{}
		err = env.Client(testutils.APIServerCluster).List(env.Ctx, roleBindings, client.InNamespace(namespace.Name))
		Expect(err).ToNot(HaveOccurred())
		Expect(roleBindings.Items).To(BeEmpty())
	})

	It("should merge subject lists from multiple roles with the same name", func() {
		env := testEnvWithAPIServerAccess("testdata", "test-08")
		authz := &openmcpv1alpha1.Authorization{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		// set the defaults for the API groups
		authz.Spec.Default()
		err = env.Client(testutils.CrateCluster).Update(env.Ctx, authz)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)
		Expect(err).ToNot(HaveOccurred())
		Expect(authz.Status.Conditions).To(ContainElements(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthorizationComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
		))

		adminClusterRoleBinding := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: openmcpv1alpha1.AdminClusterRoleBinding,
			},
		}
		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, client.ObjectKeyFromObject(adminClusterRoleBinding), adminClusterRoleBinding)
		Expect(err).ToNot(HaveOccurred())
		Expect(adminClusterRoleBinding.RoleRef.Name).To(Equal(openmcpv1alpha1.AdminClusterScopeRole))
		Expect(adminClusterRoleBinding.Subjects).To(ConsistOf(
			rbacv1.Subject{
				Kind:     rbacv1.UserKind,
				Name:     "admin",
				APIGroup: rbacv1.GroupName,
			},
			rbacv1.Subject{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "pipeline",
				Namespace: "automate",
			},
			rbacv1.Subject{
				Kind:     rbacv1.UserKind,
				Name:     "static-admin",
				APIGroup: rbacv1.GroupName,
			},
			rbacv1.Subject{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      "static-manager",
				Namespace: "openmcp-system",
			}))
	})

	It("should delete the corresponding ClusterAdmin resource when the Authorization is deleted", func() {
		var err error
		env := testEnvWithAPIServerAccess("testdata", "test-09")

		authz := &openmcpv1alpha1.Authorization{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)
		Expect(err).ToNot(HaveOccurred())
		Expect(authz.Status.Conditions).To(ContainElements(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.AuthorizationComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
		))

		clusterAdmin := &openmcpv1alpha1.ClusterAdmin{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, clusterAdmin)
		Expect(err).ToNot(HaveOccurred())

		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, authz)
		Expect(err).ToNot(HaveOccurred())

		req = testing.RequestFromObject(authz)
		_ = env.ShouldReconcile(authzReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, clusterAdmin)
		Expect(err).ToNot(HaveOccurred())

		_ = env.ShouldReconcile(authzReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, clusterAdmin)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

})

func verifyStandardClusterRole(role *rbacv1.ClusterRole) {
	Expect(role.Rules).To(HaveLen(1))

	switch role.Name {
	case openmcpv1alpha1.AdminNamespaceScopeStandardRulesRole:
		Expect(role.Labels).To(HaveKeyWithValue(aggregateToAdminLabel, "true"))
		Expect(role.Labels).To(HaveKeyWithValue(openmcpv1alpha1.AdminNamespaceScopeMatchLabel, "true"))
		Expect(role.Rules[0].Verbs).To(ConsistOf(strings.Split(adminVerbs, ",")))
		Expect(role.Rules[0].Resources).To(ConsistOf(namespaceScopedResource))
	case openmcpv1alpha1.ViewNamespaceScopeStandardRulesRole:
		Expect(role.Labels).To(HaveKeyWithValue(aggregateToViewLabel, "true"))
		Expect(role.Labels).To(HaveKeyWithValue(openmcpv1alpha1.ViewNamespaceScopeMatchLabel, "true"))
		Expect(role.Rules[0].Verbs).To(ConsistOf(strings.Split(viewVerbs, ",")))
		Expect(role.Rules[0].Resources).To(ConsistOf(namespaceScopedResource))
	case openmcpv1alpha1.AdminClusterScopeStandardRulesRole:
		Expect(role.Labels).To(HaveKeyWithValue(aggregateToAdminClusterScopedLabel, "true"))
		Expect(role.Labels).To(HaveKeyWithValue(openmcpv1alpha1.AdminClusterScopeMatchLabel, "true"))
		Expect(role.Rules[0].Verbs).To(ConsistOf(strings.Split(adminVerbs, ",")))
		Expect(role.Rules[0].Resources).To(ConsistOf(clusterScopedResource))
	case openmcpv1alpha1.ViewClusterScopeStandardClusterRole:
		Expect(role.Labels).To(HaveKeyWithValue(aggregateToViewClusterScopedLabel, "true"))
		Expect(role.Labels).To(HaveKeyWithValue(openmcpv1alpha1.ViewClusterScopeMatchLabel, "true"))
		Expect(role.Rules[0].Verbs).To(ConsistOf(strings.Split(viewVerbs, ",")))
		Expect(role.Rules[0].Resources).To(ConsistOf(clusterScopedResource))
	}
}

func verifyAggregationClusterRole(role *rbacv1.ClusterRole) {
	var labelSelectors []metav1.LabelSelector

	switch role.Name {
	case openmcpv1alpha1.AdminNamespaceScopeRole:
		labelSelectors = []metav1.LabelSelector{
			{
				MatchLabels: map[string]string{
					aggregateToAdminLabel: "true",
				},
			},
			{
				MatchLabels: map[string]string{
					openmcpv1alpha1.AdminNamespaceScopeMatchLabel: "true",
				},
			},
		}
	case openmcpv1alpha1.ViewNamespaceScopeRole:
		labelSelectors = []metav1.LabelSelector{
			{
				MatchLabels: map[string]string{
					aggregateToViewLabel: "true",
				},
			},
			{
				MatchLabels: map[string]string{
					openmcpv1alpha1.ViewNamespaceScopeMatchLabel: "true",
				},
			},
		}
	case openmcpv1alpha1.AdminClusterScopeRole:
		labelSelectors = []metav1.LabelSelector{
			{
				MatchLabels: map[string]string{
					aggregateToAdminClusterScopedLabel: "true",
				},
			},
			{
				MatchLabels: map[string]string{
					openmcpv1alpha1.AdminClusterScopeMatchLabel: "true",
				},
			},
		}
	case openmcpv1alpha1.ViewClusterScopeRole:
		labelSelectors = []metav1.LabelSelector{
			{
				MatchLabels: map[string]string{
					aggregateToViewClusterScopedLabel: "true",
				},
			},
			{
				MatchLabels: map[string]string{
					openmcpv1alpha1.ViewClusterScopeMatchLabel: "true",
				},
			},
		}
	}

	knownComponents := components.Registry.GetKnownComponents()
	for _, comp := range knownComponents {
		ls := comp.LabelSelectorsForRole(role.Name)
		if ls != nil {
			labelSelectors = append(labelSelectors, ls...)
		}
	}

	Expect(role.AggregationRule.ClusterRoleSelectors).To(HaveLen(len(labelSelectors)))
	Expect(role.AggregationRule.ClusterRoleSelectors).To(ConsistOf(labelSelectors))
}
