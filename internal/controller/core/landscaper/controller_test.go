package landscaper_test

import (
	"path"

	"github.tools.sap/CoLa/mcp-operator/internal/components"
	componentutils "github.tools.sap/CoLa/mcp-operator/internal/utils/components"

	"github.tools.sap/CoLa/mcp-operator/internal/controller/core/landscaper"

	lssv1alpha1 "github.com/gardener/landscaper-service/pkg/apis/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.tools.sap/CoLa/mcp-operator/test/matchers"

	"github.tools.sap/CoLa/controller-utils/pkg/testing"

	cconst "github.tools.sap/CoLa/mcp-operator/api/constants"
	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
	testutils "github.tools.sap/CoLa/mcp-operator/test/utils"
)

const (
	lsReconciler = "landscaper"
)

func getReconciler(c ...client.Client) reconcile.Reconciler {
	return landscaper.NewLandscaperConnector(c[0], c[1])
}

func testEnvSetup(crateObjectsPath, laasObjectsPath string, laasDynamicObjects ...client.Object) *testing.ComplexEnvironment {
	builder := testutils.DefaultTestSetupBuilder(crateObjectsPath).WithFakeClient(testutils.LaaSCoreCluster, testutils.Scheme).WithReconcilerConstructor(lsReconciler, getReconciler, testutils.CrateCluster, testutils.LaaSCoreCluster)
	if laasObjectsPath != "" {
		builder.WithInitObjectPath(testutils.LaaSCoreCluster, laasObjectsPath)
	}
	if len(laasDynamicObjects) > 0 {
		builder.WithDynamicObjectsWithStatus(testutils.LaaSCoreCluster, laasDynamicObjects...)
	}
	return builder.Build()
}

var _ = Describe("CO-1153 Landscaper Controller", func() {
	It("should set the status condition to false when there is no APIServer available", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-01"), "")

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		res := env.ShouldReconcile(lsReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).NotTo(HaveOccurred())

		Expect(ls.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForDependencies,
			}),
		))
	})

	It("should set the status condition to false when APIServer is not ready", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-02"), "")

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		res := env.ShouldReconcile(lsReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).NotTo(HaveOccurred())

		Expect(ls.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForDependencies,
			}),
		))
	})

	It("should fail to reconcile and set the status condition to false when APIServer status has no access kubeconfig", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-03"), "")

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		_ = env.ShouldNotReconcile(lsReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).NotTo(HaveOccurred())

		Expect(ls.Status.Conditions).To(ContainElements(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonDependencyStatusInvalid,
			}),
		))
	})

	It("should create a LandscaperDeployment and wait for deployment ready when APIServer is ready", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-04"), "", &lssv1alpha1.LandscaperDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
		})

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		res := env.ShouldReconcile(lsReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).NotTo(HaveOccurred())

		Expect(ls.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForLaaS,
			}),
		))

		Expect(ls.Status.LandscaperDeploymentInfo).NotTo(BeNil())

		lsDeployment := &lssv1alpha1.LandscaperDeployment{}
		err = env.Client(testutils.LaaSCoreCluster).Get(env.Ctx, types.NamespacedName{Name: ls.Status.LandscaperDeploymentInfo.Name, Namespace: ls.Status.LandscaperDeploymentInfo.Name}, lsDeployment)
		Expect(err).NotTo(HaveOccurred())

		Expect(lsDeployment.Labels).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName, ls.Name))
		Expect(lsDeployment.Labels).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace, ls.Namespace))
		Expect(lsDeployment.Spec.DataPlane).NotTo(BeNil())
		Expect(lsDeployment.Spec.DataPlane.Kubeconfig).NotTo(BeEmpty())
		Expect(lsDeployment.Spec.LandscaperConfiguration).NotTo(BeNil())
		Expect(lsDeployment.Spec.LandscaperConfiguration.Deployers).To(HaveLen(2))
		Expect(lsDeployment.Spec.LandscaperConfiguration.Deployers).To(ContainElements("helm", "manifest"))

		lsDeployment.Status.Phase = landscaper.LandscaperReadyPhase
		lsDeployment.Status.ObservedGeneration = lsDeployment.Generation
		err = env.Client(testutils.LaaSCoreCluster).Status().Update(env.Ctx, lsDeployment)
		Expect(err).NotTo(HaveOccurred())

		req = testing.RequestFromObject(ls)
		res = env.ShouldReconcile(lsReconciler, req)
		testing.ExpectNoRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).NotTo(HaveOccurred())
		Expect(ls.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
		))
	})

	It("should handle when the referenced LandscaperDeployment is not found", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-05"), "", &lssv1alpha1.LandscaperDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
		})

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		res := env.ShouldReconcile(lsReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).NotTo(HaveOccurred())

		Expect(ls.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForLaaS,
			}),
		))

		Expect(ls.Status.LandscaperDeploymentInfo).NotTo(BeNil())

		lsDeployment := &lssv1alpha1.LandscaperDeployment{}
		err = env.Client(testutils.LaaSCoreCluster).Get(env.Ctx, types.NamespacedName{Name: ls.Status.LandscaperDeploymentInfo.Name, Namespace: ls.Status.LandscaperDeploymentInfo.Name}, lsDeployment)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should handle when multiple Landscaper Deployments for the referenced are found", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-06"), path.Join("testdata", "test-06", "laas"))

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		_ = env.ShouldNotReconcile(lsReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).NotTo(HaveOccurred())

		Expect(ls.Status.Conditions).To(ContainElements(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonLaaSCoreClusterInteractionProblem,
			}),
		))
	})

	It("should handle when the LandscaperDeployment reference was lost", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-07"), path.Join("testdata", "test-07", "laas"))

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		res := env.ShouldReconcile(lsReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).NotTo(HaveOccurred())

		Expect(ls.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForLaaS,
			}),
		))

		Expect(ls.Status.LandscaperDeploymentInfo).NotTo(BeNil())
		Expect(ls.Status.LandscaperDeploymentInfo.Name).To(Equal("test1"))
		Expect(ls.Status.LandscaperDeploymentInfo.Namespace).To(Equal("test"))
	})

	It("should handle when the LandscaperDeployment has issues", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-08"), path.Join("testdata", "test-08", "laas"))

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		res := env.ShouldReconcile(lsReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).NotTo(HaveOccurred())

		Expect(ls.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.LandscaperComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForLaaS,
			}),
		))
	})

	It("should handle delete", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-09"), path.Join("testdata", "test-09", "laas"))

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		lsComp := components.Component(ls)

		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		env.ShouldReconcile(lsReconciler, req)

		// ls should still exist, since we need a second reconcile to notice that the LandscaperDeployment is gone
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).ToNot(HaveOccurred())

		env.ShouldReconcile(lsReconciler, req)
		// now it should be gone
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())

		err = env.Client(testutils.LaaSCoreCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, &lssv1alpha1.LandscaperDeployment{})
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())

		as := &openmcpv1alpha1.APIServer{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)).To(Succeed())
		auth := &openmcpv1alpha1.Authentication{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)).To(Succeed())
		authz := &openmcpv1alpha1.Authorization{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)).To(Succeed())

		Expect(componentutils.HasDepedencyFinalizer(as, lsComp.Type())).To(BeFalse())
		Expect(componentutils.HasDepedencyFinalizer(auth, lsComp.Type())).To(BeFalse())
		Expect(componentutils.HasDepedencyFinalizer(authz, lsComp.Type())).To(BeFalse())
	})

	It("should not delete when a component dependency finalizer is set", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-10"), path.Join("testdata", "test-10", "laas"))

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		lsComp := components.Component(ls)

		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		_ = env.ShouldReconcile(lsReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).ToNot(HaveOccurred())

		err = env.Client(testutils.LaaSCoreCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, &lssv1alpha1.LandscaperDeployment{})
		Expect(err).ToNot(HaveOccurred())

		as := &openmcpv1alpha1.APIServer{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)).To(Succeed())
		auth := &openmcpv1alpha1.Authentication{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)).To(Succeed())
		authz := &openmcpv1alpha1.Authorization{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)).To(Succeed())

		Expect(componentutils.HasDepedencyFinalizer(as, lsComp.Type())).To(BeTrue())
		Expect(componentutils.HasDepedencyFinalizer(auth, lsComp.Type())).To(BeTrue())
		Expect(componentutils.HasDepedencyFinalizer(authz, lsComp.Type())).To(BeTrue())
	})

	It("should handle the reconcile annotation", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-11"), "")

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		_ = env.ShouldReconcile(lsReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).ToNot(HaveOccurred())
		Expect(ls.Annotations).ToNot(HaveKeyWithValue(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueReconcile))
	})

	It("should handle the ignore annotation", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-12"), "")

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		_ = env.ShouldReconcile(lsReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).ToNot(HaveOccurred())
		Expect(ls.Annotations).To(HaveKeyWithValue(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueIgnore))

		err = env.Client(testutils.LaaSCoreCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, &lssv1alpha1.LandscaperDeployment{})
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())
	})

	It("should handle when landscaper is not found", func() {
		env := testEnvSetup("", "")

		env.ShouldReconcile(lsReconciler, testing.RequestFromObject(&openmcpv1alpha1.Landscaper{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
		}))
	})

	It("should handle delete with no LandscaperDeployment", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-13"), "")

		ls := &openmcpv1alpha1.Landscaper{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, ls)
		Expect(err).NotTo(HaveOccurred())

		lsComp := components.Component(ls)

		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, ls)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(ls)
		_ = env.ShouldReconcile(lsReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(ls), ls)
		Expect(err).To(HaveOccurred())
		Expect(errors.IsNotFound(err)).To(BeTrue())

		as := &openmcpv1alpha1.APIServer{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)).To(Succeed())
		auth := &openmcpv1alpha1.Authentication{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)).To(Succeed())
		authz := &openmcpv1alpha1.Authorization{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)).To(Succeed())

		Expect(componentutils.HasDepedencyFinalizer(as, lsComp.Type())).To(BeFalse())
		Expect(componentutils.HasDepedencyFinalizer(auth, lsComp.Type())).To(BeFalse())
		Expect(componentutils.HasDepedencyFinalizer(authz, lsComp.Type())).To(BeFalse())
	})

})
