package cloudorchestrator_test

import (
	"path"

	"github.com/openmcp-project/mcp-operator/internal/components"

	"github.com/openmcp-project/mcp-operator/internal/controller/core/cloudorchestrator"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1beta1 "github.com/openmcp-project/control-plane-operator/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/openmcp-project/mcp-operator/test/matchers"

	"github.com/openmcp-project/controller-utils/pkg/testing"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	testutils "github.com/openmcp-project/mcp-operator/test/utils"
)

const (
	coReconciler = "cloudOrchestrator"
)

func getReconciler(c ...client.Client) reconcile.Reconciler {
	return cloudorchestrator.NewCloudOrchestratorController(c[0], c[1], nil)
}

func testEnvSetup(crateObjectsPath, coObjectsPath string, coDynamicObjects ...client.Object) *testing.ComplexEnvironment {
	builder := testutils.DefaultTestSetupBuilder(crateObjectsPath).WithFakeClient(testutils.COCoreCluster, testutils.Scheme).WithReconcilerConstructor(coReconciler, getReconciler, testutils.CrateCluster, testutils.COCoreCluster)
	if coObjectsPath != "" {
		builder.WithInitObjectPath(testutils.COCoreCluster, coObjectsPath)
	}
	if len(coDynamicObjects) > 0 {
		builder.WithDynamicObjectsWithStatus(testutils.COCoreCluster, coDynamicObjects...)
	}
	return builder.Build()
}

var _ = Describe("CO-1153 CloudOrchestrator Controller", func() {
	It("should not find CloudOrchestrator resource", func() {
		env := testEnvSetup("", "")

		co := &openmcpv1alpha1.CloudOrchestrator{
			ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "test"},
		}

		req := testing.RequestFromObject(co)
		res := env.ShouldReconcile(coReconciler, req)
		Expect(res).To(Equal(reconcile.Result{}))
	})

	It("should ignore CloudOrchestrator resource due to annotation", func() {
		var err error
		env := testEnvSetup(path.Join("testdata", "test-01"), "")

		co := &openmcpv1alpha1.CloudOrchestrator{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, co)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(co)
		res := env.ShouldReconcile(coReconciler, req)
		Expect(res).To(Equal(reconcile.Result{}))
	})

	It("should not find APIServer resource", func() {
		var err error
		env := testEnvSetup(path.Join("testdata", "test-02"), "")

		co := &openmcpv1alpha1.CloudOrchestrator{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, co)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(co)
		res := env.ShouldReconcile(coReconciler, req)
		testing.ExpectRequeue(res)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(co), co)
		Expect(err).NotTo(HaveOccurred())

		Expect(co.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:    openmcpv1alpha1.CloudOrchestratorComponent.HealthyCondition(),
				Status:  openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason:  cconst.ReasonWaitingForDependencies,
				Message: "Waiting for APIServer dependency to be ready.",
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.CloudOrchestratorComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
		))
	})

	It("should fail to reconcile and set the status condition to false when APIServer has an unsupported type", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-03"), "")

		co := &openmcpv1alpha1.CloudOrchestrator{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, co)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(co)
		_ = env.ShouldNotReconcile(coReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(co), co)
		Expect(err).NotTo(HaveOccurred())

		Expect(co.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:    openmcpv1alpha1.CloudOrchestratorComponent.HealthyCondition(),
				Status:  openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason:  cconst.ReasonReconciliationError,
				Message: cconst.MessageReconciliationError,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.CloudOrchestratorComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonInvalidAPIServerType,
			}),
		))
	})

	It("should fail to reconcile and set the status condition to false when APIServer status has no access kubeconfig", func() {
		var err error
		env := testEnvSetup(path.Join("testdata", "test-04"), "")

		co := &openmcpv1alpha1.CloudOrchestrator{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, co)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(co)
		_ = env.ShouldNotReconcile(coReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(co), co)
		Expect(err).NotTo(HaveOccurred())

		Expect(co.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:    openmcpv1alpha1.CloudOrchestratorComponent.HealthyCondition(),
				Status:  openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason:  cconst.ReasonReconciliationError,
				Message: cconst.MessageReconciliationError,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.CloudOrchestratorComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonDependencyStatusInvalid,
			}),
		))
	})

	It("should create the ControlPlane resource", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-08"), "")

		co := &openmcpv1alpha1.CloudOrchestrator{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, co)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(co)
		_ = env.ShouldReconcile(coReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(co), co)
		Expect(err).NotTo(HaveOccurred())

		Expect(co.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.CloudOrchestratorComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForCloudOrchestrator,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.CloudOrchestratorComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
		))

		cp := &corev1beta1.ControlPlane{}
		err = env.Client(testutils.COCoreCluster).Get(env.Ctx, types.NamespacedName{
			Namespace: "",
			Name:      "test--test",
		}, cp)
		Expect(err).NotTo(HaveOccurred())
		Expect(cp.Spec.Target.Kubeconfig).NotTo(BeNil())
		Expect(cp.Spec.Crossplane.Version).To(Equal("1.17.0")) // configured
		Expect(cp.Spec.Crossplane.Providers[0]).NotTo(BeNil())
		Expect(cp.Spec.Crossplane.Providers[0].Name).To(Equal("provider-kubernetes")) // configured
		Expect(cp.Spec.Crossplane.Providers[0].Version).To(Equal("0.14.1"))           // configured
		Expect(cp.Spec.ExternalSecretsOperator.Version).To(Equal("0.10.0"))           // configured
		Expect(cp.Spec.BTPServiceOperator.Version).To(Equal("0.6.0"))                 // configured
		Expect(cp.Spec.CertManager.Version).To(Equal("1.16.1"))                       // configured automatically
		Expect(cp.Spec.Flux.Version).To(Equal("3.2.0"))                               // configured
		Expect(cp.Spec.Kyverno.Version).To(Equal("3.2.7"))                            // configured
	})

	It("should delete the ControlPlane resource and then the CO Resource", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-06"), "")
		// adding ControlPlane resource to Core cluster to simulate the deletion of the ControlPlane resource
		cp := &corev1beta1.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test--test",
				Namespace:  "",
				Finalizers: []string{"COre.orchestrate.cloud.sap"},
			},
			Status: corev1beta1.ControlPlaneStatus{ComponentsEnabled: 1, ComponentsHealthy: 0},
		}
		err = env.Client(testutils.COCoreCluster).Create(env.Ctx, cp)
		Expect(err).NotTo(HaveOccurred())

		co := &openmcpv1alpha1.CloudOrchestrator{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, co)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(co)
		_ = env.ShouldReconcile(coReconciler, req)

		// delete CloudOrchestrator resource
		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, co)
		Expect(err).ToNot(HaveOccurred())

		// trigger new reconcile
		req = testing.RequestFromObject(co)
		res := env.ShouldReconcile(coReconciler, req)
		Expect(res.RequeueAfter > 0).To(BeTrue()) // expecting requeue, since ControlPlane resource is not deleted yet

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(co), co)
		Expect(err).ToNot(HaveOccurred())
		Expect(co.Finalizers).To(ContainElement(openmcpv1alpha1.CloudOrchestratorComponent.Finalizer()))

		Expect(co.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.CloudOrchestratorComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.CloudOrchestratorComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				Reason: cconst.ReasonComponentIsInDeletion,
			}),
		))

		cp = &corev1beta1.ControlPlane{}
		err = env.Client(testutils.COCoreCluster).Get(env.Ctx, types.NamespacedName{
			Namespace: "",
			Name:      "test--test",
		}, cp)
		Expect(apierrors.IsNotFound(err)).To(BeFalse()) // ControlPlane resource is still there

		as := &openmcpv1alpha1.APIServer{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)).To(Succeed())
		auth := &openmcpv1alpha1.Authentication{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, auth)).To(Succeed())
		authz := &openmcpv1alpha1.Authorization{}
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, authz)).To(Succeed())
		coComp := components.Component(co)
		Expect(as.Finalizers).To(ContainElement(coComp.Type().DependencyFinalizer()))    // Finalizer on APIServer resource is still there
		Expect(auth.Finalizers).To(ContainElement(coComp.Type().DependencyFinalizer()))  // Finalizer on Authentication resource is still there
		Expect(authz.Finalizers).To(ContainElement(coComp.Type().DependencyFinalizer())) // Finalizer on Authorization resource is still there

		// --- SUB TEST: simulating a requeue for the CO resource to check if the finalizer at the APIServer resource is removed ---
		cp = &corev1beta1.ControlPlane{}
		err = env.Client(testutils.COCoreCluster).Get(env.Ctx, types.NamespacedName{
			Namespace: "",
			Name:      "test--test",
		}, cp)
		Expect(err).NotTo(HaveOccurred())
		controllerutil.RemoveFinalizer(cp, "COre.orchestrate.cloud.sap") // removing finalizer on ControlPlane resource to simulate successful deletion
		err = env.Client(testutils.COCoreCluster).Update(env.Ctx, cp)
		Expect(err).NotTo(HaveOccurred())

		req = testing.RequestFromObject(co)
		res = env.ShouldReconcile(coReconciler, req)
		Expect(res.RequeueAfter == 0).To(BeTrue()) // expecting no requeue

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(co), co)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())

		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
		Expect(as.Finalizers).ToNot(ContainElement(coComp.Type().DependencyFinalizer()))
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(auth), auth)).To(Succeed())
		Expect(auth.Finalizers).ToNot(ContainElement(coComp.Type().DependencyFinalizer()))
		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(authz), authz)).To(Succeed())
		Expect(authz.Finalizers).ToNot(ContainElement(coComp.Type().DependencyFinalizer()))
	})

	It("should not be deleted when it has a dependency finalizer", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-07"), "")

		co := &openmcpv1alpha1.CloudOrchestrator{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, co)
		Expect(err).NotTo(HaveOccurred())

		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, co)
		Expect(err).ToNot(HaveOccurred())

		req := testing.RequestFromObject(co)
		_ = env.ShouldReconcile(coReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(co), co)
		Expect(err).NotTo(HaveOccurred())
		Expect(co.Finalizers).To(ContainElement(openmcpv1alpha1.CloudOrchestratorComponent.Finalizer()))
		Expect(co.Finalizers).To(ContainElement("dependency." + openmcpv1alpha1.BaseDomain + "/other_comp"))
	})

	It("should update the ControlPlane resource since the configuration got changed", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-05"), "")

		co := &openmcpv1alpha1.CloudOrchestrator{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, co)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(co)
		_ = env.ShouldReconcile(coReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(co), co)
		Expect(err).NotTo(HaveOccurred())

		cp := &corev1beta1.ControlPlane{}
		err = env.Client(testutils.COCoreCluster).Get(env.Ctx, types.NamespacedName{
			Namespace: "",
			Name:      "test--test",
		}, cp)
		Expect(err).NotTo(HaveOccurred())
		Expect(cp.Spec.Target.Kubeconfig).NotTo(BeNil())
		Expect(cp.Spec.Crossplane.Version).To(Equal("1.17.0")) // configured
		Expect(cp.Spec.ExternalSecretsOperator).To(BeNil())

		// Update CO Spec
		co.Spec.BTPServiceOperator = &openmcpv1alpha1.BTPServiceOperatorConfig{
			Version: "0.6.0",
		}
		co.Spec.ExternalSecretsOperator = &openmcpv1alpha1.ExternalSecretsOperatorConfig{
			Version: "0.10.0",
		}
		err = env.Client(testutils.CrateCluster).Update(env.Ctx, co)
		Expect(err).NotTo(HaveOccurred())

		req2 := testing.RequestFromObject(co)
		_ = env.ShouldReconcile(coReconciler, req2)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(co), co)
		Expect(err).NotTo(HaveOccurred())

		Expect(co.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.CloudOrchestratorComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForCloudOrchestrator,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.CloudOrchestratorComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
		))

		cp = &corev1beta1.ControlPlane{}
		err = env.Client(testutils.COCoreCluster).Get(env.Ctx, types.NamespacedName{
			Namespace: "",
			Name:      "test--test",
		}, cp)
		Expect(err).NotTo(HaveOccurred())
		Expect(cp.Spec.Target.Kubeconfig).NotTo(BeNil())
		Expect(cp.Spec.Crossplane.Version).To(Equal("1.17.0")) // configured
		Expect(cp.Spec.ExternalSecretsOperator).ToNot(BeNil()) // updated
		Expect(cp.Spec.ExternalSecretsOperator.Version).To(Equal("0.10.0"))
		Expect(cp.Spec.BTPServiceOperator).ToNot(BeNil()) // updated
		Expect(cp.Spec.BTPServiceOperator.Version).To(Equal("0.6.0"))
		Expect(cp.Spec.CertManager).ToNot(BeNil()) // updated automatically
		Expect(cp.Spec.CertManager.Version).To(Equal("1.16.1"))
	})

	It("should remove Crossplane configuration from the ControlPlane resource", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-05"), "")

		co := &openmcpv1alpha1.CloudOrchestrator{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, co)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(co)
		_ = env.ShouldReconcile(coReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(co), co)
		Expect(err).NotTo(HaveOccurred())

		cp := &corev1beta1.ControlPlane{}
		err = env.Client(testutils.COCoreCluster).Get(env.Ctx, types.NamespacedName{
			Namespace: "",
			Name:      "test--test",
		}, cp)
		Expect(err).NotTo(HaveOccurred())
		Expect(cp.Spec.Target.Kubeconfig).NotTo(BeNil())
		Expect(cp.Spec.Crossplane.Version).To(Equal("1.17.0")) // configured
		Expect(cp.Spec.ExternalSecretsOperator).To(BeNil())

		// Disable Crossplane
		co.Spec.Crossplane = nil
		err = env.Client(testutils.CrateCluster).Update(env.Ctx, co)
		Expect(err).NotTo(HaveOccurred())

		req2 := testing.RequestFromObject(co)
		_ = env.ShouldReconcile(coReconciler, req2)

		cp = &corev1beta1.ControlPlane{}
		err = env.Client(testutils.COCoreCluster).Get(env.Ctx, types.NamespacedName{
			Namespace: "",
			Name:      "test--test",
		}, cp)
		Expect(err).NotTo(HaveOccurred())
		Expect(cp.Spec.Target.Kubeconfig).NotTo(BeNil())
		Expect(cp.Spec.Crossplane).To(BeNil())
	})

	It("should update the ControlPlane resource even when in deletion", func() {
		var err error

		env := testEnvSetup(path.Join("testdata", "test-09"), "")

		co := &openmcpv1alpha1.CloudOrchestrator{}
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, co)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(co)
		_ = env.ShouldReconcile(coReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(co), co)
		Expect(err).NotTo(HaveOccurred())

		Expect(co.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.CloudOrchestratorComponent.HealthyCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason: cconst.ReasonWaitingForCloudOrchestrator,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   openmcpv1alpha1.CloudOrchestratorComponent.ReconciliationCondition(),
				Status: openmcpv1alpha1.ComponentConditionStatusTrue,
			}),
		))

		cp := &corev1beta1.ControlPlane{}
		err = env.Client(testutils.COCoreCluster).Get(env.Ctx, types.NamespacedName{
			Namespace: "",
			Name:      "test--test",
		}, cp)
		Expect(err).NotTo(HaveOccurred())
		Expect(cp.Spec.Target.Kubeconfig).NotTo(BeNil())
		// set a finalizer to simulate deletion
		controllerutil.AddFinalizer(cp, "core.orchestrate.cloud.sap")
		err = env.Client(testutils.COCoreCluster).Update(env.Ctx, cp)
		Expect(err).NotTo(HaveOccurred())

		// Delete CO resource
		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, co)
		Expect(err).ToNot(HaveOccurred())
		_ = env.ShouldReconcile(coReconciler, req)

		// check that the ControlPlane resource has a deletion timestamp
		err = env.Client(testutils.COCoreCluster).Get(env.Ctx, types.NamespacedName{
			Namespace: "",
			Name:      "test--test",
		}, cp)
		Expect(err).NotTo(HaveOccurred())
		Expect(cp.DeletionTimestamp).ToNot(BeNil())

		// Update CO resource to trigger an update on the ControlPlane resource
		err = env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, co)
		Expect(err).NotTo(HaveOccurred())
		co.Spec.Kyverno = &openmcpv1alpha1.KyvernoConfig{
			Version: "8.8.8",
		}
		err = env.Client(testutils.CrateCluster).Update(env.Ctx, co)
		Expect(err).NotTo(HaveOccurred())
		_ = env.ShouldReconcile(coReconciler, req)

		err = env.Client(testutils.COCoreCluster).Get(env.Ctx, types.NamespacedName{
			Namespace: "",
			Name:      "test--test",
		}, cp)
		Expect(err).NotTo(HaveOccurred())
		Expect(cp.Spec.Kyverno).ToNot(BeNil())
		Expect(cp.Spec.Kyverno.Version).To(Equal("8.8.8"))
	})
})
