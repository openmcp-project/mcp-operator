package gardener_test

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	gardenv1beta1 "github.com/openmcp-project/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"
	apiserverconfig "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/config"
	"github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/handler/gardener"
	testutils "github.com/openmcp-project/mcp-operator/test/utils"
)

func setDefaultAuditLog(gc *gardener.GardenerConnector) {
	common := *gc.Common
	common.DefaultAuditLog = &apiserverconfig.DefaultAuditLogConfiguration{
		Type: "standard", TenantID: "platform-tenant", ServiceURL: "https://platform.example.com",
		SecretRef: openmcpv1alpha1.NamespacedObjectReference{Name: "central-secret", Namespace: "operator-secrets"},
		PolicyRef: openmcpv1alpha1.NamespacedObjectReference{Name: "central-policy", Namespace: "operator-policies"},
	}
	gc.Common = &common
}

func createDefaultAuditLogSources() (*corev1.Secret, *corev1.ConfigMap) {
	// Synthetic test credentials only; no production credentials are used.
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "central-secret", Namespace: "operator-secrets"}, Type: corev1.SecretTypeOpaque, Data: map[string][]byte{"user": []byte("test-only-user"), "password": []byte("test-only-password")}}
	policy := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "central-policy", Namespace: "operator-policies"}, Data: map[string]string{"policy": "platform-policy"}}
	Expect(env.Client(testutils.CrateCluster).Create(env.Ctx, secret)).To(Succeed())
	Expect(env.Client(testutils.CrateCluster).Create(env.Ctx, policy)).To(Succeed())
	return secret, policy
}

func getTestAuditShoot() *gardenv1beta1.Shoot {
	sh := &gardenv1beta1.Shoot{}
	Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test", Namespace: "garden-test"}, sh)).To(Succeed())
	return sh
}

func expectAuditEndpoint(sh *gardenv1beta1.Shoot, tenant, url string) {
	var provider map[string]string
	for _, extension := range sh.Spec.Extensions {
		if extension.Type == "shoot-auditlog-service" {
			Expect(json.Unmarshal(extension.ProviderConfig.Raw, &provider)).To(Succeed())
		}
	}
	Expect(provider).To(HaveKeyWithValue("tenantID", tenant))
	Expect(provider).To(HaveKeyWithValue("serviceURL", url))
	Expect(provider).To(HaveKeyWithValue("secretReferenceName", "auditlog-credentials"))
	Expect(sh.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name).To(Equal(sh.Name + "--auditlog-policy"))
	Expect(sh.Spec.Resources).To(ContainElement(gardenv1beta1.NamedResourceReference{Name: "auditlog-credentials", ResourceRef: autoscalingv1.CrossVersionObjectReference{APIVersion: "v1", Kind: "Secret", Name: sh.Name + "--auditlog-credentials"}}))
}

var _ = Describe("Default audit logging", func() {
	for mode, initHandler := range map[string]func(openmcpv1alpha1.APIServerType, string, ...string) (*gardener.GardenerConnector, *openmcpv1alpha1.APIServer){"single": initGardenerHandlerTestSingle, "multi": initGardenerHandlerTestMulti} {
		Context(fmt.Sprintf("config %s", mode), func() {
			for _, apiServerType := range []openmcpv1alpha1.APIServerType{openmcpv1alpha1.Gardener, openmcpv1alpha1.GardenerDedicated} {
				Context(fmt.Sprintf("type %s", apiServerType), func() {
					for _, noGardener := range []bool{false, true} {
						It(fmt.Sprintf("should use central sources and survive repeated reconciliation (no gardener=%t)", noGardener), func() {
							gc, as := initHandler(apiServerType, "", "testdata", "connector", "apiserver-06.yaml")
							as.Spec.GardenerConfig.AuditLog = nil
							if noGardener {
								as.Spec.GardenerConfig = nil
							}
							before := as.DeepCopy()
							setDefaultAuditLog(gc)
							secret, policy := createDefaultAuditLogSources()
							for range 2 {
								_, _, _, err := gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
								Expect(err).ToNot(HaveOccurred())
								expectAuditEndpoint(getTestAuditShoot(), "platform-tenant", "https://platform.example.com")
							}
							Expect(as).To(Equal(before))
							copiedSecret := &corev1.Secret{}
							copiedPolicy := &corev1.ConfigMap{}
							Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test--auditlog-credentials", Namespace: "garden-test"}, copiedSecret)).To(Succeed())
							Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test--auditlog-policy", Namespace: "garden-test"}, copiedPolicy)).To(Succeed())
							Expect(copiedSecret.Data).To(Equal(secret.Data))
							Expect(copiedPolicy.Data).To(Equal(policy.Data))
							Expect(apierrors.IsNotFound(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: "central-secret", Namespace: as.Namespace}, &corev1.Secret{}))).To(BeTrue())
							secret.Data["password"] = []byte("rotated-test-only-password")
							policy.Data["policy"] = "updated-platform-policy"
							Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, secret)).To(Succeed())
							Expect(env.Client(testutils.CrateCluster).Update(env.Ctx, policy)).To(Succeed())
							_, _, _, err := gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
							Expect(err).ToNot(HaveOccurred())
							Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: copiedSecret.Name, Namespace: copiedSecret.Namespace}, copiedSecret)).To(Succeed())
							Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: copiedPolicy.Name, Namespace: copiedPolicy.Namespace}, copiedPolicy)).To(Succeed())
							Expect(copiedSecret.Data).To(Equal(secret.Data))
							Expect(copiedPolicy.Data).To(Equal(policy.Data))
						})
					}

					It("should apply the default to a newly created shoot", func() {
						gc, as := initHandler(apiServerType, "", "testdata", "connector", "apiserver-05.yaml")
						as.Spec.GardenerConfig = nil
						setDefaultAuditLog(gc)
						createDefaultAuditLogSources()
						_, updateStatus, _, err := gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
						Expect(err).ToNot(HaveOccurred())
						Expect(updateStatus(&as.Status)).To(Succeed())
						manifest, manifestErr := as.Status.GardenerStatus.GetShoot()
						Expect(manifestErr).ToNot(HaveOccurred())
						sh := &gardenv1beta1.Shoot{}
						Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: manifest.GetName(), Namespace: manifest.GetNamespace()}, sh)).To(Succeed())
						expectAuditEndpoint(sh, "platform-tenant", "https://platform.example.com")
					})

					It("should prefer explicit settings and switch to the default when removed", func() {
						gc, as := initHandler(apiServerType, "", "testdata", "connector", "apiserver-06.yaml")
						explicit := as.Spec.GardenerConfig.AuditLog.DeepCopy()
						setDefaultAuditLog(gc)
						// Central resources do not exist yet: the explicit config must still work.
						_, _, _, err := gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
						Expect(err).ToNot(HaveOccurred())
						expectAuditEndpoint(getTestAuditShoot(), explicit.TenantID, explicit.ServiceURL)
						createDefaultAuditLogSources()
						as.Spec.GardenerConfig.AuditLog = nil
						_, _, _, err = gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
						Expect(err).ToNot(HaveOccurred())
						expectAuditEndpoint(getTestAuditShoot(), "platform-tenant", "https://platform.example.com")
						// Reinstating explicit customer settings must replace the fallback again.
						as.Spec.GardenerConfig.AuditLog = explicit
						_, _, _, err = gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
						Expect(err).ToNot(HaveOccurred())
						expectAuditEndpoint(getTestAuditShoot(), explicit.TenantID, explicit.ServiceURL)
						as.Spec.GardenerConfig.AuditLog = nil
						gc.Common.DefaultAuditLog = nil
						_, _, _, err = gc.HandleCreateOrUpdate(env.Ctx, as, nil)
						Expect(err).ToNot(HaveOccurred())
						Expect(getTestAuditShoot().Spec.Kubernetes.KubeAPIServer.AuditConfig).To(BeNil())
						Expect(apierrors.IsNotFound(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test--auditlog-credentials", Namespace: "garden-test"}, &corev1.Secret{}))).To(BeTrue())
					})

					It("should report invalid explicit config without using the default or changing the shoot", func() {
						gc, as := initHandler(apiServerType, "", "testdata", "connector", "apiserver-06.yaml")
						setDefaultAuditLog(gc)
						createDefaultAuditLogSources()
						as.Spec.GardenerConfig.AuditLog = &openmcpv1alpha1.AuditLogConfig{}
						before := getTestAuditShoot()
						_, _, _, err := gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
						Expect(err).To(HaveOccurred())
						Expect(getTestAuditShoot()).To(Equal(before))
					})

					It("should not fall back when explicit source resources are missing", func() {
						gc, as := initHandler(apiServerType, "", "testdata", "connector", "apiserver-06.yaml")
						setDefaultAuditLog(gc)
						createDefaultAuditLogSources()
						as.Spec.GardenerConfig.AuditLog.SecretRef.Name = "missing-customer-secret"
						before := getTestAuditShoot()
						_, _, _, err := gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
						Expect(err).To(HaveOccurred())
						Expect(getTestAuditShoot()).To(Equal(before))
					})

					It("should report missing default sources without changing the existing shoot", func() {
						gc, as := initHandler(apiServerType, "", "testdata", "connector", "apiserver-06.yaml")
						as.Spec.GardenerConfig.AuditLog = nil
						setDefaultAuditLog(gc)
						before := getTestAuditShoot()
						_, _, _, err := gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
						Expect(err).To(HaveOccurred())
						Expect(getTestAuditShoot()).To(Equal(before))
					})

					It("should delete generated resources without deleting central sources or applying the fallback", func() {
						gc, as := initHandler(apiServerType, "", "testdata", "connector", "apiserver-06.yaml")
						as.Spec.GardenerConfig.AuditLog = nil
						setDefaultAuditLog(gc)
						secret, policy := createDefaultAuditLogSources()
						_, _, _, err := gc.HandleCreateOrUpdate(env.Ctx, as, env.Client(testutils.CrateCluster))
						Expect(err).ToNot(HaveOccurred())
						before := as.DeepCopy()
						_, _, _, err = gc.HandleDelete(env.Ctx, as, nil)
						Expect(err).ToNot(HaveOccurred())
						Expect(as).To(Equal(before))
						Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, secret)).To(Succeed())
						Expect(env.Client(testutils.CrateCluster).Get(env.Ctx, types.NamespacedName{Name: policy.Name, Namespace: policy.Namespace}, policy)).To(Succeed())
						Expect(apierrors.IsNotFound(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test--auditlog-credentials", Namespace: "garden-test"}, &corev1.Secret{}))).To(BeTrue())
						Expect(apierrors.IsNotFound(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "test--auditlog-policy", Namespace: "garden-test"}, &corev1.ConfigMap{}))).To(BeTrue())
					})
				})
			}
		})
	}
})
