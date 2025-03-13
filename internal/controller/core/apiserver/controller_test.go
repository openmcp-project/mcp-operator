package apiserver_test

import (
	"context"
	"fmt"

	"github.tools.sap/CoLa/mcp-operator/internal/controller/core/apiserver"
	apiserverhandler "github.tools.sap/CoLa/mcp-operator/internal/controller/core/apiserver/handler"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.tools.sap/CoLa/mcp-operator/test/matchers"

	"github.com/openmcp-project/controller-utils/pkg/testing"

	cconst "github.tools.sap/CoLa/mcp-operator/api/constants"
	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.tools.sap/CoLa/mcp-operator/api/errors"
	testutils "github.tools.sap/CoLa/mcp-operator/test/utils"
)

func getReconciler(c ...client.Client) reconcile.Reconciler {
	r, err := apiserver.NewAPIServerProvider(constructorContext, c[0], defaultConfig)
	Expect(err).NotTo(HaveOccurred())
	r.FakeHandler = fakeHandler
	return r
}

const (
	apiServerReconciler    = "apiserver"
	mockReadyConditionType = "mockReady"
)

func testEnvSetup(testDirPathSegments ...string) *testing.ComplexEnvironment {
	return testutils.DefaultTestSetupBuilder(testDirPathSegments...).WithFakeClient(testutils.APIServerCluster, testutils.Scheme).WithReconcilerConstructor(apiServerReconciler, getReconciler, testutils.CrateCluster).Build()
}

func mockReadyConditions(ready bool) []openmcpv1alpha1.ComponentCondition {
	return []openmcpv1alpha1.ComponentCondition{
		{
			Type:               mockReadyConditionType,
			Status:             openmcpv1alpha1.ComponentConditionStatusFromBool(ready),
			LastTransitionTime: metav1.Now(),
		},
	}
}

var _ = Describe("CO-1153 APIServer Controller", func() {
	It("should do nothing if the reconciled resource is not found", func() {
		env := testEnvSetup()

		env.ShouldReconcile(apiServerReconciler, reconcile.Request{NamespacedName: types.NamespacedName{Name: "test", Namespace: "test"}})
	})

	It("should handle the ignore annotation", func() {
		env := testEnvSetup("testdata", "test-01")

		as := &openmcpv1alpha1.APIServer{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())
		old := as.DeepCopy()

		req := testing.RequestFromObject(as)
		_ = env.ShouldReconcile(apiServerReconciler, req)

		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
		Expect(as.Status.Conditions).To(HaveLen(0))
		Expect(old).To(Equal(as))
	})

	It("should handle the reconcile annotation", func() {
		env := testEnvSetup("testdata", "test-02")

		as := &openmcpv1alpha1.APIServer{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())
		Expect(as.Annotations).To(HaveKeyWithValue(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueReconcile))

		fakeHandler.MockHandleCreateOrUpdateCall(func(ctx context.Context, dp *openmcpv1alpha1.APIServer, crateClient client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
			return reconcile.Result{}, nil, mockReadyConditions(true), nil
		})
		req := testing.RequestFromObject(as)
		_ = env.ShouldReconcile(apiServerReconciler, req)

		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
		Expect(as.Annotations).ToNot(HaveKeyWithValue(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueReconcile))
	})

	It("should not be deleted when it has a dependency finalizer", func() {
		env := testEnvSetup("testdata", "test-03")

		as := &openmcpv1alpha1.APIServer{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		err = env.Client(testutils.CrateCluster).Delete(env.Ctx, as)
		Expect(err).ToNot(HaveOccurred())

		fakeHandler.MockHandleCreateOrUpdateCall(func(ctx context.Context, dp *openmcpv1alpha1.APIServer, crateClient client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
			return reconcile.Result{}, nil, mockReadyConditions(true), nil
		})
		req := testing.RequestFromObject(as)
		_ = env.ShouldReconcile(apiServerReconciler, req)

		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
		Expect(as.Finalizers).To(ContainElement(openmcpv1alpha1.LandscaperComponent.DependencyFinalizer()))
	})

	It("should fail for unknown APIServer types", func() {
		env := testEnvSetup("testdata", "test-04")

		as := &openmcpv1alpha1.APIServer{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		req := testing.RequestFromObject(as)
		_ = env.ShouldNotReconcile(apiServerReconciler, req)
	})

	It("should add the APIServer finalizer if it does not yet exist", func() {
		env := testEnvSetup("testdata", "test-05")

		as := &openmcpv1alpha1.APIServer{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		fakeHandler.MockHandleCreateOrUpdateCall(func(ctx context.Context, dp *openmcpv1alpha1.APIServer, crateClient client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
			return reconcile.Result{}, nil, mockReadyConditions(true), nil
		})
		req := testing.RequestFromObject(as)
		_ = env.ShouldReconcile(apiServerReconciler, req)

		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
		Expect(as.Finalizers).To(ContainElement(openmcpv1alpha1.APIServerComponent.Finalizer()))
	})

	It("should propagate the error from HandleCreateOrUpdate to the APIServer status", func() {
		env := testEnvSetup("testdata", "test-05")

		as := &openmcpv1alpha1.APIServer{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		fakeHandler.MockHandleCreateOrUpdateCall(func(ctx context.Context, dp *openmcpv1alpha1.APIServer, crateClient client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
			return reconcile.Result{}, nil, mockReadyConditions(false), openmcperrors.WithReason(fmt.Errorf("test error"), "test reason")
		})
		req := testing.RequestFromObject(as)
		_ = env.ShouldNotReconcile(apiServerReconciler, req)

		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
		Expect(as.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:    openmcpv1alpha1.APIServerComponent.HealthyCondition(),
				Status:  openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason:  cconst.ReasonReconciliationError,
				Message: cconst.MessageReconciliationError,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:    openmcpv1alpha1.APIServerComponent.ReconciliationCondition(),
				Status:  openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason:  "test reason",
				Message: "test error",
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   mockReadyConditionType,
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
			}),
		))
	})

	It("should not remove the finalizer if the deletion did not finish", func() {
		env := testEnvSetup("testdata", "test-06")

		as := &openmcpv1alpha1.APIServer{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		fakeHandler.MockHandleDeleteCall(func(ctx context.Context, dp *openmcpv1alpha1.APIServer, crateClient client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
			return reconcile.Result{}, nil, mockReadyConditions(false), nil
		})
		req := testing.RequestFromObject(as)
		_ = env.ShouldReconcile(apiServerReconciler, req)

		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
		Expect(as.Finalizers).To(ContainElement(openmcpv1alpha1.APIServerComponent.Finalizer()))
	})

	It("should remove the finalizer if the deletion finished", func() {
		env := testEnvSetup("testdata", "test-06")

		as := &openmcpv1alpha1.APIServer{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		fakeHandler.MockHandleDeleteCall(func(ctx context.Context, dp *openmcpv1alpha1.APIServer, crateClient client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
			return reconcile.Result{}, nil, mockReadyConditions(true), nil
		})
		req := testing.RequestFromObject(as)
		_ = env.ShouldReconcile(apiServerReconciler, req)

		err = env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("should propagate the error from HandleDelete to the APIServer status", func() {
		env := testEnvSetup("testdata", "test-06")

		as := &openmcpv1alpha1.APIServer{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		fakeHandler.MockHandleDeleteCall(func(ctx context.Context, dp *openmcpv1alpha1.APIServer, crateClient client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
			return reconcile.Result{}, nil, mockReadyConditions(false), openmcperrors.WithReason(fmt.Errorf("test error"), "test reason")
		})
		req := testing.RequestFromObject(as)
		_ = env.ShouldNotReconcile(apiServerReconciler, req)

		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
		Expect(as.Status.Conditions).To(ConsistOf(
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:    openmcpv1alpha1.APIServerComponent.HealthyCondition(),
				Status:  openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason:  cconst.ReasonReconciliationError,
				Message: cconst.MessageReconciliationError,
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:    openmcpv1alpha1.APIServerComponent.ReconciliationCondition(),
				Status:  openmcpv1alpha1.ComponentConditionStatusFalse,
				Reason:  "test reason",
				Message: "test error",
			}),
			MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
				Type:   mockReadyConditionType,
				Status: openmcpv1alpha1.ComponentConditionStatusFalse,
			}),
		))
	})

	It("should call the UpdateStatusFunc if returned by HandleCreateOrUpdate", func() {
		env := testEnvSetup("testdata", "test-05")

		as := &openmcpv1alpha1.APIServer{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		fakeHandler.MockHandleCreateOrUpdateCall(func(ctx context.Context, dp *openmcpv1alpha1.APIServer, crateClient client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
			return reconcile.Result{}, func(dps *openmcpv1alpha1.APIServerStatus) error {
				dps.GardenerStatus = &openmcpv1alpha1.GardenerStatus{
					Shoot: &runtime.RawExtension{
						Raw: []byte(`{"apiVersion":"garden.sapcloud.io/v1beta1","kind":"Shoot","metadata":{"name":"foo","namespace":"bar"}}`),
					},
				}
				dps.ExternalAPIServerStatus = &openmcpv1alpha1.ExternalAPIServerStatus{
					Endpoint:             "https://k8s-external.ondemand.com",
					ServiceAccountIssuer: "https://k8s-sa.ondemand.com",
				}
				return nil
			}, mockReadyConditions(true), nil
		})
		req := testing.RequestFromObject(as)
		_ = env.ShouldReconcile(apiServerReconciler, req)

		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
		Expect(as.Status.GardenerStatus).ToNot(BeNil())
		Expect(as.Status.GardenerStatus.Shoot).ToNot(BeNil())
		uShoot, err := as.Status.GardenerStatus.GetShoot()
		Expect(err).NotTo(HaveOccurred())
		Expect(uShoot.GetName()).To(Equal("foo"))
		Expect(uShoot.GetNamespace()).To(Equal("bar"))
		Expect(as.Status.ExternalAPIServerStatus.Endpoint).To(Equal("https://k8s-external.ondemand.com"))
		Expect(as.Status.ExternalAPIServerStatus.ServiceAccountIssuer).To(Equal("https://k8s-sa.ondemand.com"))
	})

	It("should call the UpdateStatusFunc if returned by HandleDelete", func() {
		env := testEnvSetup("testdata", "test-06")

		as := &openmcpv1alpha1.APIServer{}
		err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
		Expect(err).NotTo(HaveOccurred())

		fakeHandler.MockHandleDeleteCall(func(ctx context.Context, dp *openmcpv1alpha1.APIServer, crateClient client.Client) (reconcile.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
			return reconcile.Result{}, func(dps *openmcpv1alpha1.APIServerStatus) error {
				dps.GardenerStatus = &openmcpv1alpha1.GardenerStatus{
					Shoot: &runtime.RawExtension{
						Raw: []byte(`{"apiVersion":"garden.sapcloud.io/v1beta1","kind":"Shoot","metadata":{"name":"foo","namespace":"bar"}}`),
					},
				}
				dps.ExternalAPIServerStatus = &openmcpv1alpha1.ExternalAPIServerStatus{
					Endpoint:             "https://k8s-external.ondemand.com",
					ServiceAccountIssuer: "https://k8s-sa.ondemand.com",
				}
				return nil
			}, mockReadyConditions(false), nil
		})
		req := testing.RequestFromObject(as)
		_ = env.ShouldReconcile(apiServerReconciler, req)

		Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
		Expect(as.Status.GardenerStatus).ToNot(BeNil())
		Expect(as.Status.GardenerStatus.Shoot).ToNot(BeNil())
		uShoot, err := as.Status.GardenerStatus.GetShoot()
		Expect(err).NotTo(HaveOccurred())
		Expect(uShoot.GetName()).To(Equal("foo"))
		Expect(uShoot.GetNamespace()).To(Equal("bar"))
		Expect(as.Status.ExternalAPIServerStatus.Endpoint).To(Equal("https://k8s-external.ondemand.com"))
		Expect(as.Status.ExternalAPIServerStatus.ServiceAccountIssuer).To(Equal("https://k8s-sa.ondemand.com"))
	})

})
