package clusteradmin_test

import (
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"

	"github.com/openmcp-project/controller-utils/pkg/testing"

	"github.com/openmcp-project/mcp-operator/internal/controller/core/authorization/clusteradmin"
	"github.com/openmcp-project/mcp-operator/internal/controller/core/authorization/config"
	testutils "github.com/openmcp-project/mcp-operator/test/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	clusterAdminReconciler = "ClusterAdmin"
)

func getReconciler(c ...client.Client) reconcile.Reconciler {
	return clusteradmin.NewClusterAdminReconciler(c[0], &config.AuthorizationConfig{
		ClusterAdmin: config.ClusterAdmin{
			ActiveDuration: metav1.Duration{Duration: 1 * time.Second},
		},
	})
}

func testEnvWithAPIServerAccess(testDataPathSegments ...string) *testing.ComplexEnvironment {
	env := testutils.DefaultTestSetupBuilder(testDataPathSegments...).
		WithFakeClient(testutils.APIServerCluster, testutils.Scheme).
		WithReconcilerConstructor(clusterAdminReconciler, getReconciler, testutils.CrateCluster).
		WithDynamicObjectsWithStatus(testutils.CrateCluster).
		Build()
	env.Reconcilers[clusterAdminReconciler].(*clusteradmin.ClusterAdminReconciler).
		SetAPIServerAccess(&testutils.TestAPIServerAccess{Client: env.Client(testutils.APIServerCluster)})

	return env
}

var _ = Describe("CO-1153 ClusterAdmin Controller", func() {
	It("should create a cluster role binding for the cluster admin", func() {
		env := testEnvWithAPIServerAccess("testdata", "test-01")

		authz := &openmcpv1alpha1.Authorization{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		ca := &openmcpv1alpha1.ClusterAdmin{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ca)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(ca)
		res := env.ShouldReconcile(clusterAdminReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ca), ca)
		Expect(err).ToNot(HaveOccurred())

		Expect(ca.Status.Active).To(BeTrue())
		Expect(ca.Status.Activated).ToNot(BeNil())
		Expect(ca.Status.Expiration).ToNot(BeNil())

		// Check if the cluster role binding was created
		crb := &rbacv1.ClusterRoleBinding{}
		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, types.NamespacedName{Name: openmcpv1alpha1.ClusterAdminRoleBinding}, crb)
		Expect(err).ToNot(HaveOccurred())

		// Check if the cluster role binding was created with the correct subjects
		Expect(crb.Subjects).To(HaveLen(1))
		Expect(crb.Subjects[0].Kind).To(Equal("User"))
		Expect(crb.Subjects[0].Name).To(Equal("admin"))

		// Check if the cluster role binding was created with the correct role ref
		Expect(crb.RoleRef.APIGroup).To(Equal("rbac.authorization.k8s.io"))
		Expect(crb.RoleRef.Kind).To(Equal("ClusterRole"))
		Expect(crb.RoleRef.Name).To(Equal(openmcpv1alpha1.ClusterAdminRole))

		Eventually(func() bool {
			res := env.ShouldReconcile(clusterAdminReconciler, req)
			err = env.Client(testutils.APIServerCluster).Get(env.Ctx, types.NamespacedName{Name: openmcpv1alpha1.ClusterAdminRoleBinding}, crb)
			return errors.IsNotFound(err) && res.Requeue == false && res.RequeueAfter == 0
		}, 2*time.Second, 100*time.Millisecond).Should(BeTrue())

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ca), ca)
		Expect(err).ToNot(HaveOccurred())

		Expect(ca.Status.Active).To(BeFalse())

		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, types.NamespacedName{Name: openmcpv1alpha1.ClusterAdminRoleBinding}, crb)
		Expect(errors.IsNotFound(err)).To(BeTrue())

		// it should not activate again
		res = env.ShouldReconcile(clusterAdminReconciler, req)
		testing.ExpectNoRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ca), ca)
		Expect(err).ToNot(HaveOccurred())

		Expect(ca.Status.Active).To(BeFalse())

		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, types.NamespacedName{Name: openmcpv1alpha1.ClusterAdminRoleBinding}, crb)
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("should deactivate the cluster admin if the clusteradmin object is deleted while being active", func() {
		env := testEnvWithAPIServerAccess("testdata", "test-01")

		authz := &openmcpv1alpha1.Authorization{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)
		Expect(err).ToNot(HaveOccurred())

		ca := &openmcpv1alpha1.ClusterAdmin{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ca)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(ca)
		res := env.ShouldReconcile(clusterAdminReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ca), ca)
		Expect(err).ToNot(HaveOccurred())

		Expect(ca.Status.Active).To(BeTrue())
		Expect(ca.Status.Activated).ToNot(BeNil())
		Expect(ca.Status.Expiration).ToNot(BeNil())

		crb := &rbacv1.ClusterRoleBinding{}
		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, types.NamespacedName{Name: openmcpv1alpha1.ClusterAdminRoleBinding}, crb)
		Expect(err).ToNot(HaveOccurred())

		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, ca)
		Expect(err).ToNot(HaveOccurred())

		res = env.ShouldReconcile(clusterAdminReconciler, req)
		testing.ExpectNoRequeue(res)

		err = env.Client(testutils.APIServerCluster).Get(env.Ctx, types.NamespacedName{Name: openmcpv1alpha1.ClusterAdminRoleBinding}, crb)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("should fail if there is no authorization resource", func() {
		env := testEnvWithAPIServerAccess("testdata", "test-02")

		ca := &openmcpv1alpha1.ClusterAdmin{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ca)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(ca)
		env.ShouldNotReconcile(clusterAdminReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ca), ca)
		Expect(err).ToNot(HaveOccurred())

		Expect(ca.Status.Active).To(BeFalse())
	})

	It("should fail if there is no apiserver resource", func() {
		env := testEnvWithAPIServerAccess("testdata", "test-03")

		ca := &openmcpv1alpha1.ClusterAdmin{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ca)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(ca)
		env.ShouldNotReconcile(clusterAdminReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ca), ca)
		Expect(err).ToNot(HaveOccurred())

		Expect(ca.Status.Active).To(BeFalse())
	})

	It("should requeue when apiserver has no admin access", func() {
		env := testEnvWithAPIServerAccess("testdata", "test-04")

		ca := &openmcpv1alpha1.ClusterAdmin{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ca)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(ca)
		res := env.ShouldReconcile(clusterAdminReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ca), ca)
		Expect(err).ToNot(HaveOccurred())

		Expect(ca.Status.Active).To(BeFalse())
	})
})
