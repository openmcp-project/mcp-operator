package gardener_test

import (
	"fmt"
	"time"

	componentutils "github.com/openmcp-project/mcp-operator/internal/utils/components"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/openmcp-project/mcp-operator/test/matchers"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	gardenv1beta1 "github.com/openmcp-project/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"
	"github.com/openmcp-project/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/handler/gardener"
	apiserverutils "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/utils"
	testutils "github.com/openmcp-project/mcp-operator/test/utils"
)

var (
	defaultAPIServerType = openmcpv1alpha1.Gardener
)

var _ = Describe("APIServer Gardener Conversion", func() {

	for cfgType, initGardenerHandlerTest := range map[string]func(openmcpv1alpha1.APIServerType, string, ...string) (*gardener.GardenerConnector, *openmcpv1alpha1.APIServer){
		"single": initGardenerHandlerTestSingle,
		"multi":  initGardenerHandlerTestMulti,
	} {

		Context(fmt.Sprintf("Config Type: %s", cfgType), func() {

			Context("GetShoot", func() {

				It("should fetch the shoot from its reference in the APIServer status", func() {
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-01.yaml")
					sh1 := &gardenv1beta1.Shoot{}
					Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "garden-test"}, sh1)).To(Succeed())
					sh2 := &gardenv1beta1.Shoot{}
					sh2.SetName("test2")
					sh2.SetNamespace(sh1.Namespace)
					sh2.SetLabels(sh1.GetLabels())
					sh2.SetAnnotations(sh1.GetAnnotations())
					sh2.Spec = *sh1.Spec.DeepCopy()
					sh2.Status = *sh1.Status.DeepCopy()
					Expect(env.Client(gardenCluster).Create(env.Ctx, sh2)).To(Succeed())
					shoot, rerr := gc.GetShoot(env.Ctx, as, false)
					Expect(rerr).ToNot(HaveOccurred())
					Expect(shoot).To(Equal(sh1))
				})

				It("should fetch the shoot based on its labels if the APIServer status is lost", func() {
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-02.yaml")
					compare := &gardenv1beta1.Shoot{}
					Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "garden-test"}, compare)).To(Succeed())
					shoot, rerr := gc.GetShoot(env.Ctx, as, false)
					Expect(rerr).ToNot(HaveOccurred())
					Expect(shoot).To(Equal(compare))
				})

				It("should return nil if the shoot did never exist", func() {
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-03.yaml")
					shoot, rerr := gc.GetShoot(env.Ctx, as, false)
					Expect(rerr).ToNot(HaveOccurred())
					Expect(shoot).To(BeNil())
				})

				It("should return an error if more than one shoot matches the labels", func() {
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-02.yaml")
					sh1 := &gardenv1beta1.Shoot{}
					Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "garden-test"}, sh1)).To(Succeed())
					sh2 := &gardenv1beta1.Shoot{}
					sh2.SetName("test2")
					sh2.SetNamespace(sh1.Namespace)
					sh2.SetLabels(sh1.GetLabels())
					sh2.SetAnnotations(sh1.GetAnnotations())
					sh2.Spec = *sh1.Spec.DeepCopy()
					sh2.Status = *sh1.Status.DeepCopy()
					Expect(env.Client(gardenCluster).Create(env.Ctx, sh2)).To(Succeed())
					_, rerr := gc.GetShoot(env.Ctx, as, false)
					Expect(rerr).To(MatchError(ContainSubstring("more than one")))
				})

			})

			Context("HandleCreateOrUpdate", func() {

				It("should add admin access if the shoot is ready", func() {
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-04.yaml")
					res, usf, cons, err := gc.HandleCreateOrUpdate(env.Ctx, as, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(cons).To(ConsistOf(
						MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
							Type:   openmcpv1alpha1.APIServerComponent.HealthyCondition(),
							Status: openmcpv1alpha1.ComponentConditionStatusTrue,
						}),
					))
					Expect(res.RequeueAfter).To(BeNumerically(">=", apiserverutils.DefaultAdminAccessValidityTime/2))
					Expect(res.RequeueAfter).To(BeNumerically("<=", apiserverutils.DefaultAdminAccessValidityTime))
					Expect(usf).ToNot(BeNil())
					Expect(as.Status.AdminAccess).To(BeNil())
					Expect(usf(&as.Status)).To(Succeed())
					Expect(as.Status.AdminAccess).ToNot(BeNil())
				})

				It("should not add admin access if the shoot is not ready", func() {
					sh := &gardenv1beta1.Shoot{}
					Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "garden-test"}, sh)).To(Succeed())
					old := sh.DeepCopy()
					sh.Status.Conditions[0].Status = gardenv1beta1.ConditionFalse
					Expect(env.Client(gardenCluster).Status().Patch(env.Ctx, sh, client.MergeFrom(old))).To(Succeed())
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-04.yaml")
					res, usf, cons, err := gc.HandleCreateOrUpdate(env.Ctx, as, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(cons).To(ConsistOf(
						MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
							Type:   openmcpv1alpha1.APIServerComponent.HealthyCondition(),
							Status: openmcpv1alpha1.ComponentConditionStatusFalse,
						}),
					))
					Expect(res.RequeueAfter).To(BeNumerically(">", 0))
					Expect(res.RequeueAfter).To(BeNumerically("<=", 5*time.Minute))
					Expect(usf).ToNot(BeNil())
					Expect(as.Status.AdminAccess).To(BeNil())
					Expect(usf(&as.Status)).To(Succeed())
					Expect(as.Status.AdminAccess).To(BeNil())
				})

				It("should expose endpoint and serviceaccount issuer if exposed in shoot status", func() {
					sh := &gardenv1beta1.Shoot{}
					Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "garden-test"}, sh)).To(Succeed())
					expectedEndpoint := ""
					expectedServiceAccountIssuer := ""
					for _, aa := range sh.Status.AdvertisedAddresses {
						if aa.Name == constants.AdvertisedAddressExternal {
							expectedEndpoint = aa.URL
							continue
						}
						if aa.Name == constants.AdvertisedAddressServiceAccountIssuer {
							expectedServiceAccountIssuer = aa.URL
							continue
						}
					}
					Expect(expectedEndpoint).ToNot(BeEmpty(), "test prerequisite not fulfilled: shoot status should advertise an 'external' address")
					Expect(expectedServiceAccountIssuer).ToNot(BeEmpty(), "test prerequisite not fulfilled: shoot status should advertise a 'serviceAccountIssuer' address")
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-04.yaml")
					_, usf, _, err := gc.HandleCreateOrUpdate(env.Ctx, as, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(usf).ToNot(BeNil())
					Expect(as.Status.ExternalAPIServerStatus.Endpoint).To(BeEmpty())
					Expect(as.Status.ExternalAPIServerStatus.ServiceAccountIssuer).To(BeEmpty())
					Expect(usf(&as.Status)).To(Succeed())
					Expect(as.Status.ExternalAPIServerStatus.Endpoint).To(Equal(expectedEndpoint))
					Expect(as.Status.ExternalAPIServerStatus.ServiceAccountIssuer).To(Equal(expectedServiceAccountIssuer))
				})

				It("should create a new shoot if none exists", func() {
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-05.yaml")
					res, usf, cons, err := gc.HandleCreateOrUpdate(env.Ctx, as, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(cons).To(ConsistOf(
						MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
							Type:   openmcpv1alpha1.APIServerComponent.HealthyCondition(),
							Status: openmcpv1alpha1.ComponentConditionStatusFalse,
						}),
					))
					Expect(res.RequeueAfter).To(BeNumerically(">", 0))
					Expect(res.RequeueAfter).To(BeNumerically("<=", 5*time.Minute))
					Expect(usf).ToNot(BeNil())
					Expect(as.Status.AdminAccess).To(BeNil())
					Expect(as.Status.GardenerStatus).To(BeNil())
					Expect(usf(&as.Status)).To(Succeed())
					Expect(as.Status.AdminAccess).To(BeNil())
					Expect(as.Status.GardenerStatus).ToNot(BeNil())
					sh1 := &gardenv1beta1.Shoot{}
					uShoot, err2 := as.Status.GardenerStatus.GetShoot()
					Expect(err2).ToNot(HaveOccurred())
					Expect(uShoot).ToNot(BeNil())
					Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: uShoot.GetName(), Namespace: uShoot.GetNamespace()}, sh1)).To(Succeed())
				})

				It("should fail if the to-be-created shoot already exists, but is lacking the required labels", func() {
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-05.yaml")
					sh := &gardenv1beta1.Shoot{}
					sh.SetName(gardener.ComputeShootName(&as.ObjectMeta, "test"))
					sh.SetNamespace("garden-test")
					Expect(env.Client(gardenCluster).Create(env.Ctx, sh)).To(Succeed())
					_, _, _, err := gc.HandleCreateOrUpdate(env.Ctx, as, nil)
					Expect(err).To(HaveOccurred())
					Expect(err).To(MatchError(ContainSubstring("already exists")))
				})

				It("should copy audit log resources to the Garden namespace", func() {
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-06.yaml")
					_, _, cons, errr := gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
					Expect(errr).ToNot(HaveOccurred())
					Expect(cons).To(ConsistOf(
						MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
							Type:   openmcpv1alpha1.APIServerComponent.HealthyCondition(),
							Status: openmcpv1alpha1.ComponentConditionStatusTrue,
						}),
					))

					cmGarden := &corev1.ConfigMap{}
					err := env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test--auditlog-policy", Namespace: "garden-test"}, cmGarden)
					Expect(err).ToNot(HaveOccurred())
					secretGarden := &corev1.Secret{}
					err = env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test--auditlog-credentials", Namespace: "garden-test"}, secretGarden)
					Expect(err).ToNot(HaveOccurred())
				})

				It("should update the audit log resources in the Garden namespace when the shoot already exists with the correct auditlog configuration", func() {
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-07.yaml")
					_, _, cons, errr := gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
					Expect(errr).ToNot(HaveOccurred())
					Expect(cons).To(ConsistOf(
						MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
							Type:   openmcpv1alpha1.APIServerComponent.HealthyCondition(),
							Status: openmcpv1alpha1.ComponentConditionStatusTrue,
						}),
					))

					cmGarden := &corev1.ConfigMap{}
					err := env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test-auditlog--auditlog-policy", Namespace: "garden-test"}, cmGarden)
					Expect(err).ToNot(HaveOccurred())
					secretGarden := &corev1.Secret{}
					err = env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test-auditlog--auditlog-credentials", Namespace: "garden-test"}, secretGarden)
					Expect(err).ToNot(HaveOccurred())

					sh := &gardenv1beta1.Shoot{}
					err = env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test-auditlog", Namespace: "garden-test"}, sh)
					Expect(err).ToNot(HaveOccurred())
					Expect(sh.GetAnnotations()).To(HaveKeyWithValue(constants.GardenerOperation, constants.GardenerOperationReconcile))
				})

			})

			Context("HandleDelete", func() {

				It("should delete the shoot if it exists", func() {
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-04.yaml")
					sh := &gardenv1beta1.Shoot{}
					Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "garden-test"}, sh)).To(Succeed())
					res, _, cons, err := gc.HandleDelete(env.Ctx, as, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(cons).To(ConsistOf(
						MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
							Type:   openmcpv1alpha1.APIServerComponent.HealthyCondition(),
							Status: openmcpv1alpha1.ComponentConditionStatusFalse,
						}),
					))
					Expect(res.RequeueAfter).To(BeNumerically(">", 0))
					Expect(res.RequeueAfter).To(BeNumerically("<=", 5*time.Minute))
					err2 := env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "garden-test"}, sh)
					Expect(err2).To(HaveOccurred())
					Expect(apierrors.IsNotFound(err2)).To(BeTrue())
				})

				It("should return ready and remove the shoot reference if the shoot does not exist", func() {
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-04.yaml")
					gls, _, err := gc.LandscapeConfiguration()
					Expect(err).ToNot(HaveOccurred())
					sh := &gardenv1beta1.Shoot{}
					Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "garden-test"}, sh)).To(Succeed())
					Expect(componentutils.PatchAnnotation(env.Ctx, gls.Client, sh, gardener.GardenerDeletionConfirmationAnnotation, "true", componentutils.ANNOTATION_OVERWRITE)).To(Succeed())
					Expect(env.Client(gardenCluster).Delete(env.Ctx, sh)).To(Succeed())
					err2 := env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "garden-test"}, sh)
					Expect(err2).To(HaveOccurred())
					Expect(apierrors.IsNotFound(err2)).To(BeTrue())
					res, usf, cons, err := gc.HandleDelete(env.Ctx, as, nil)
					Expect(err).ToNot(HaveOccurred())
					Expect(cons).To(ConsistOf(
						MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
							Type:   openmcpv1alpha1.APIServerComponent.HealthyCondition(),
							Status: openmcpv1alpha1.ComponentConditionStatusTrue,
						}),
					))
					Expect(res).To(Equal(ctrl.Result{}))
					Expect(usf).ToNot(BeNil())
					Expect(as.Status.GardenerStatus).ToNot(BeNil())
					Expect(as.Status.GardenerStatus.Shoot).ToNot(BeNil())
					Expect(usf(&as.Status)).To(Succeed())
					Expect(as.Status.GardenerStatus.Shoot).To(BeNil())
				})

				It("should create and then delete the audit log resources along with the shoot", func() {
					gc, as := initGardenerHandlerTest(defaultAPIServerType, "", "testdata", "connector", "apiserver-06.yaml")
					_, _, cons, errr := gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
					Expect(errr).ToNot(HaveOccurred())
					Expect(cons).To(ConsistOf(
						MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
							Type:   openmcpv1alpha1.APIServerComponent.HealthyCondition(),
							Status: openmcpv1alpha1.ComponentConditionStatusTrue,
						}),
					))

					_, _, cons, errr = gc.HandleDelete(env.Ctx, as, env.Client(testutils.CrateCluster))
					Expect(errr).ToNot(HaveOccurred())
					Expect(cons).To(ConsistOf(
						MatchComponentCondition(openmcpv1alpha1.ComponentCondition{
							Type:   openmcpv1alpha1.APIServerComponent.HealthyCondition(),
							Status: openmcpv1alpha1.ComponentConditionStatusFalse,
						}),
					))

					cmGarden := &corev1.ConfigMap{}
					err := env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test--auditlog-policy", Namespace: "garden-test"}, cmGarden)
					Expect(apierrors.IsNotFound(err)).To(BeTrue())
					secretGarden := &corev1.Secret{}
					err = env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test--auditlog-credentials", Namespace: "garden-test"}, secretGarden)
					Expect(apierrors.IsNotFound(err)).To(BeTrue())

					sh := &gardenv1beta1.Shoot{}
					err = env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "garden-test"}, sh)
					Expect(apierrors.IsNotFound(err)).To(BeTrue())
				})

			})

		})

	}

})
