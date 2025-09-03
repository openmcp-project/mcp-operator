package apiserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	mcpocfg "github.com/openmcp-project/mcp-operator/internal/config"
	"github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver"
	apiserverhandler "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/handler"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/openmcp-project/mcp-operator/test/matchers"

	"github.com/openmcp-project/controller-utils/pkg/testing"
	clustersv1alpha1 "github.com/openmcp-project/openmcp-operator/api/clusters/v1alpha1"
	commonapi "github.com/openmcp-project/openmcp-operator/api/common"
	openmcpclusterutils "github.com/openmcp-project/openmcp-operator/lib/utils"

	gardenv1beta1 "github.com/openmcp-project/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
	testutils "github.com/openmcp-project/mcp-operator/test/utils"
)

func getReconciler(c ...client.Client) reconcile.Reconciler {
	r, err := apiserver.NewAPIServerProvider(constructorContext, c[0], c[1], defaultConfig)
	Expect(err).NotTo(HaveOccurred())
	r.FakeHandler = fakeHandler
	return r
}

const (
	apiServerReconciler    = "apiserver"
	mockReadyConditionType = "mockReady"
)

func testEnvSetup(testDirPathSegments ...string) *testing.ComplexEnvironment {
	return testutils.DefaultTestSetupBuilder(testDirPathSegments...).
		WithFakeClient(testutils.APIServerCluster, testutils.Scheme).
		WithFakeClient(testutils.LaaSCoreCluster, testutils.Scheme).
		WithDynamicObjectsWithStatus(testutils.LaaSCoreCluster, &clustersv1alpha1.AccessRequest{}, &clustersv1alpha1.ClusterRequest{}, &clustersv1alpha1.Cluster{}).
		WithReconcilerConstructor(apiServerReconciler, getReconciler, testutils.CrateCluster, testutils.LaaSCoreCluster).
		Build()
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
				dps.ExternalAPIServerStatus = openmcpv1alpha1.ExternalAPIServerStatus{
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
				dps.ExternalAPIServerStatus = openmcpv1alpha1.ExternalAPIServerStatus{
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

	Context("v2", func() {

		BeforeEach(func() {
			mcpocfg.Config.Architecture.APIServer.Version = openmcpv1alpha1.ArchitectureV2
		})

		It("should create a ClusterRequest and AccessRequest instead of a Shoot", func() {
			env := testEnvSetup("testdata", "test-07")

			as := &openmcpv1alpha1.APIServer{}
			err := env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "test"}, as)
			Expect(err).NotTo(HaveOccurred())

			req := testing.RequestFromObject(as)
			rr := env.ShouldReconcile(apiServerReconciler, req)
			Expect(rr.RequeueAfter).To(BeNumerically(">", 0))

			cr := &clustersv1alpha1.ClusterRequest{}
			cr.Name = as.Name
			cr.Namespace, err = openmcpclusterutils.StableMCPNamespace(as.Name, as.Namespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(env.Client(testutils.LaaSCoreCluster).Get(env.Ctx, client.ObjectKeyFromObject(cr), cr)).To(Succeed())

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
			Expect(as.Status.Conditions).To(ContainElements(
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionClusterRequestGranted,
					Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				}),
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionClusterReady,
					Status: openmcpv1alpha1.ComponentConditionStatusUnknown,
				}),
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionAccessRequestGranted,
					Status: openmcpv1alpha1.ComponentConditionStatusUnknown,
				}),
			))

			// mock Cluster and ClusterRequest status
			cluster := &clustersv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-namespace",
				},
				Spec: clustersv1alpha1.ClusterSpec{
					Purposes: []string{"mcp"},
					Tenancy:  clustersv1alpha1.TENANCY_EXCLUSIVE,
				},
			}
			Expect(env.Client(testutils.LaaSCoreCluster).Create(env.Ctx, cluster)).To(Succeed())

			cr.Status.Phase = clustersv1alpha1.REQUEST_GRANTED
			cr.Status.Cluster = &commonapi.ObjectReference{
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			}
			Expect(env.Client(testutils.LaaSCoreCluster).Status().Update(env.Ctx, cr)).To(Succeed())

			// reconcile again, should now get further
			rr = env.ShouldReconcile(apiServerReconciler, req)
			Expect(rr.RequeueAfter).To(BeNumerically(">", 0))

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
			Expect(as.Status.Conditions).To(ContainElements(
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionClusterRequestGranted,
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionClusterReady,
					Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				}),
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionAccessRequestGranted,
					Status: openmcpv1alpha1.ComponentConditionStatusUnknown,
				}),
			))

			// mock Cluster status
			dummyShoot := &gardenv1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-shoot",
					Namespace: "test-namespace",
				},
				Spec: gardenv1beta1.ShootSpec{
					CloudProfileName: ptr.To("gcp"),
					Purpose:          ptr.To(gardenv1beta1.ShootPurposeProduction),
					Region:           "europe",
				},
			}
			dummyShootJson, err := json.Marshal(dummyShoot)
			Expect(err).NotTo(HaveOccurred())
			cluster.Status = clustersv1alpha1.ClusterStatus{
				ProviderStatus: &runtime.RawExtension{
					Raw: dummyShootJson,
				},
			}
			cluster.Status.Phase = clustersv1alpha1.CLUSTER_PHASE_READY
			Expect(env.Client(testutils.LaaSCoreCluster).Status().Update(env.Ctx, cluster)).To(Succeed())

			// reconcile again, should now get further
			rr = env.ShouldReconcile(apiServerReconciler, req)
			Expect(rr.RequeueAfter).To(BeNumerically(">", 0))

			ar := &clustersv1alpha1.AccessRequest{}
			ar.Name = as.Name
			ar.Namespace = cr.Namespace
			Expect(env.Client(testutils.LaaSCoreCluster).Get(env.Ctx, client.ObjectKeyFromObject(ar), ar)).To(Succeed())

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
			Expect(as.Status.Conditions).To(ContainElements(
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionClusterRequestGranted,
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionClusterReady,
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionAccessRequestGranted,
					Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				}),
			))

			// mock AccessRequest status and secret
			creationTime := time.Now()
			expirationTime := time.Now().Add(24 * time.Hour)
			access := &corev1.Secret{}
			access.Name = "test-access-secret"
			access.Namespace = ar.Namespace
			access.Data = map[string][]byte{
				"kubeconfig":          []byte("fake"),
				"creationTimestamp":   []byte(strconv.FormatInt(creationTime.Unix(), 10)),
				"expirationTimestamp": []byte(strconv.FormatInt(expirationTime.Unix(), 10)),
			}
			Expect(env.Client(testutils.LaaSCoreCluster).Create(env.Ctx, access)).To(Succeed())

			ar.Status.Phase = clustersv1alpha1.REQUEST_GRANTED
			ar.Status.SecretRef = &commonapi.ObjectReference{
				Name:      access.Name,
				Namespace: access.Namespace,
			}
			Expect(env.Client(testutils.LaaSCoreCluster).Status().Update(env.Ctx, ar)).To(Succeed())

			// reconcile again, should now get further
			rr = env.ShouldReconcile(apiServerReconciler, req)

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
			Expect(as.Status.Conditions).To(ContainElements(
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionClusterRequestGranted,
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionClusterReady,
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionAccessRequestGranted,
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
			))

			Expect(as.Status.AdminAccess).ToNot(BeNil())
			Expect(as.Status.AdminAccess.CreationTimestamp.Time).To(BeTemporally("~", creationTime, 1*time.Second))
			Expect(as.Status.AdminAccess.ExpirationTimestamp.Time).To(BeTemporally("~", expirationTime, 1*time.Second))
			Expect(as.Status.AdminAccess.Kubeconfig).To(BeEquivalentTo("fake"))

			reconcileAt := creationTime.Add(time.Duration(float64(expirationTime.Sub(creationTime)) * 0.85))
			Expect(rr.RequeueAfter).To(BeNumerically("~", time.Until(reconcileAt), 1*time.Second))

			// add dummy finalizers to the ClusterRequest and AccessRequest to verify the deletion flow
			cr.Finalizers = append(cr.Finalizers, "dummy")
			Expect(env.Client(testutils.LaaSCoreCluster).Update(env.Ctx, cr)).To(Succeed())
			ar.Finalizers = append(ar.Finalizers, "dummy")
			Expect(env.Client(testutils.LaaSCoreCluster).Update(env.Ctx, ar)).To(Succeed())

			// delete the APIServer, should not be deleted because of the finalizers
			Expect(env.Client(testutils.CrateCluster).Delete(env.Ctx, as)).To(Succeed())
			rr = env.ShouldReconcile(apiServerReconciler, req)
			Expect(rr.RequeueAfter).To(BeNumerically(">", 0))

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
			Expect(as.Status.Conditions).To(ContainElements(
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionAccessRequestDeleted,
					Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				}),
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionClusterRequestDeleted,
					Status: openmcpv1alpha1.ComponentConditionStatusUnknown,
				}),
			))

			// remove the AccessRequest finalizers and reconcile again
			Expect(env.Client(testutils.LaaSCoreCluster).Get(env.Ctx, client.ObjectKeyFromObject(ar), ar)).To(Succeed())
			Expect(ar.DeletionTimestamp).ToNot(BeZero())
			ar.Finalizers = nil
			Expect(env.Client(testutils.LaaSCoreCluster).Update(env.Ctx, ar)).To(Succeed())
			rr = env.ShouldReconcile(apiServerReconciler, req)
			Expect(rr.RequeueAfter).To(BeNumerically(">", 0))

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(Succeed())
			Expect(as.Status.Conditions).To(ContainElements(
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionAccessRequestDeleted,
					Status: openmcpv1alpha1.ComponentConditionStatusTrue,
				}),
				MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
					Type:   cconst.ConditionClusterRequestDeleted,
					Status: openmcpv1alpha1.ComponentConditionStatusFalse,
				}),
			))

			// remove the ClusterRequest finalizers and reconcile again
			Expect(env.Client(testutils.LaaSCoreCluster).Get(env.Ctx, client.ObjectKeyFromObject(cr), cr)).To(Succeed())
			Expect(cr.DeletionTimestamp).ToNot(BeZero())
			cr.Finalizers = nil
			Expect(env.Client(testutils.LaaSCoreCluster).Update(env.Ctx, cr)).To(Succeed())
			rr = env.ShouldReconcile(apiServerReconciler, req)
			Expect(rr.RequeueAfter).ToNot(BeNumerically(">", 0))

			Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, client.ObjectKeyFromObject(as), as)).To(MatchError(apierrors.IsNotFound, "IsNotFound"))
		})

	})

})
