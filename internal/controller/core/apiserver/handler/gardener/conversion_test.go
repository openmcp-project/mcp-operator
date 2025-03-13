package gardener_test

import (
	"fmt"
	"net"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"
	"sigs.k8s.io/yaml"

	"github.tools.sap/CoLa/mcp-operator/internal/utils"
	"github.tools.sap/CoLa/mcp-operator/internal/utils/region"

	"github.tools.sap/CoLa/mcp-operator/internal/controller/core/apiserver/handler/gardener"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openmcp-project/controller-utils/pkg/testing"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
	gardenawsv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/external/gardener-extension-provider-aws/pkg/apis/aws/v1alpha1"
	gardenv1beta1 "github.tools.sap/CoLa/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("APIServer Gardener Conversion", func() {

	for cfgType, initGardenerHandlerTest := range map[string]func(openmcpv1alpha1.APIServerType, string, ...string) (*gardener.GardenerConnector, *openmcpv1alpha1.APIServer){
		"single": initGardenerHandlerTestSingle,
		"multi":  initGardenerHandlerTestMulti,
	} {

		Context(fmt.Sprintf("Config Type: %s", cfgType), func() {

			var flavors []string
			if cfgType == "single" {
				flavors = []string{"default/default"}
			} else {
				flavors = []string{"default/gcp", "default/aws"}
			}

			for _, flavor := range flavors {

				Context(fmt.Sprintf("Flavor: %s", flavor), func() {

					for _, apiServerType := range []openmcpv1alpha1.APIServerType{openmcpv1alpha1.Gardener, openmcpv1alpha1.GardenerDedicated} {

						Context(fmt.Sprintf("Type: %s", string(apiServerType)), func() {

							It("should convert a APIServer to a Shoot (generic region)", func() {
								gc, apiServer := initGardenerHandlerTest(apiServerType, flavor, "testdata", "conversion", "apiserver-01.yaml")
								_, gcfg, err := gc.LandscapeConfiguration(flavor)
								Expect(err).ToNot(HaveOccurred())
								shoot := &gardenv1beta1.Shoot{}
								Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())

								Expect(shoot.Namespace).To(Equal(gcfg.ProjectNamespace))

								// spec.region
								mapper := region.GetPredefinedMapperByCloudprovider(gcfg.ProviderType)
								Expect(mapper).ToNot(BeNil())
								regions, err := region.GetClosestRegions(*apiServer.Spec.DesiredRegion, mapper, sets.KeySet(gcfg.ValidRegions).UnsortedList(), true)
								Expect(err).ToNot(HaveOccurred())
								Expect(regions).ToNot(BeEmpty())
								Expect(regions).To(ContainElement(shoot.Spec.Region), "shoot region is not in the list of closest regions")

								commonShootValidation(gc, apiServer, shoot, flavor)
							})

							It("should convert a APIServer to a Shoot (region overwrite)", func() {
								gc, apiServer := initGardenerHandlerTest(apiServerType, flavor, "testdata", "conversion", "apiserver-02.yaml")
								_, gcfg, err := gc.LandscapeConfiguration(flavor)
								Expect(err).ToNot(HaveOccurred())
								shoot := &gardenv1beta1.Shoot{}
								Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())

								Expect(shoot.Namespace).To(Equal(gcfg.ProjectNamespace))

								// spec.region
								Expect(shoot.Spec.Region).To(Equal(apiServer.Spec.GardenerConfig.Region))

								commonShootValidation(gc, apiServer, shoot, flavor)
							})

							It("should convert a APIServer to a Shoot (region fallback)", func() {
								gc, apiServer := initGardenerHandlerTest(apiServerType, flavor, "testdata", "conversion", "apiserver-03.yaml")
								_, gcfg, err := gc.LandscapeConfiguration(flavor)
								Expect(err).ToNot(HaveOccurred())
								shoot := &gardenv1beta1.Shoot{}
								Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())

								Expect(shoot.Namespace).To(Equal(gcfg.ProjectNamespace))

								// spec.region
								Expect(shoot.Spec.Region).To(Equal(gcfg.DefaultRegion))

								commonShootValidation(gc, apiServer, shoot, flavor)
							})

							It("should not overwrite immutable fields of the shoot", func() {
								gc, apiServer := initGardenerHandlerTest(apiServerType, flavor, "testdata", "conversion", "apiserver-01.yaml")
								gc.APIServerType = apiServerType
								apiServer.Spec.Type = apiServerType
								shoot := &gardenv1beta1.Shoot{}
								Expect(env.Client(gardenCluster).Get(env.Ctx, types.NamespacedName{Name: "modified", Namespace: "garden-test"}, shoot)).To(Succeed())
								compare := shoot.DeepCopy()
								Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())

								Expect(shoot.Spec.Region).To(Equal(compare.Spec.Region))
								Expect(shoot.Spec.CloudProfileName).To(Equal(compare.Spec.CloudProfileName))
								Expect(shoot.Spec.SecretBindingName).To(Equal(compare.Spec.SecretBindingName))
								Expect(shoot.Spec.Provider.Type).To(Equal(compare.Spec.Provider.Type))
								Expect(shoot.Spec.Networking.Type).To(Equal(compare.Spec.Networking.Type))
							})

							It("should convert the AuditLog configuration to the Shoot spec", func() {
								gc, apiServer := initGardenerHandlerTest(apiServerType, flavor, "testdata", "conversion", "apiserver-04.yaml")
								_, gcfg, err := gc.LandscapeConfiguration(flavor)
								Expect(err).ToNot(HaveOccurred())
								shoot := &gardenv1beta1.Shoot{}
								Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())

								Expect(shoot.Namespace).To(Equal(gcfg.ProjectNamespace))

								Expect(shoot.Spec.Resources[0].Name).To(Equal("auditlog-credentials"))
								Expect(shoot.Spec.Resources[0].ResourceRef.Kind).To(Equal("Secret"))
								Expect(shoot.Spec.Resources[0].ResourceRef.Name).To(Equal(utils.PrefixWithNamespace(shoot.Name, "auditlog-credentials")))

								Expect(shoot.Spec.Extensions).To(HaveLen(2))
								Expect(shoot.Spec.Extensions[1].ProviderConfig.Raw).ToNot(BeEmpty())

								Expect(shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name).To(Equal(utils.PrefixWithNamespace(shoot.Name, "auditlog-policy")))

								commonShootValidation(gc, apiServer, shoot, flavor)
							})

							It("should remove the AuditLog configuration from the Shoot spec", func() {
								gc, apiServer := initGardenerHandlerTest(apiServerType, flavor, "testdata", "conversion", "apiserver-05.yaml")

								shoot := &gardenv1beta1.Shoot{}
								Expect(testing.LoadObject(shoot, "testdata", "conversion", "shoot-05.yaml")).To(Succeed())
								oldNs := shoot.Namespace

								Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())

								Expect(shoot.Namespace).To(Equal(oldNs))

								Expect(shoot.Spec.Resources).To(BeEmpty())
								Expect(shoot.Spec.Extensions).To(HaveLen(2))
								Expect(shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig).To(BeNil())
							})

							It("should set the high availability configuration to node", func() {
								gc, apiServer := initGardenerHandlerTest(apiServerType, flavor, "testdata", "conversion", "apiserver-07.yaml")
								if strings.HasSuffix(flavor, "aws") {
									// aws doesn't have the 'europe-west3' region that is hardcoded in the apiserver config
									apiServer.Spec.GardenerConfig.Region = "eu-west-1"
								}
								_, gcfg, err := gc.LandscapeConfiguration(flavor)
								Expect(err).ToNot(HaveOccurred())
								shoot := &gardenv1beta1.Shoot{}
								Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())

								Expect(shoot.Namespace).To(Equal(gcfg.ProjectNamespace))

								Expect(shoot.Spec.ControlPlane).ToNot(BeNil())
								Expect(shoot.Spec.ControlPlane.HighAvailability).ToNot(BeNil())
								Expect(shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type).To(
									Equal(gardenv1beta1.FailureToleranceType(openmcpv1alpha1.HighAvailabilityFailureToleranceNode)))

								if apiServerType == openmcpv1alpha1.GardenerDedicated {
									Expect(shoot.Spec.Provider.Workers).To(HaveLen(1))
									Expect(shoot.Spec.Provider.Workers[0].Zones).To(HaveLen(1))
									Expect(shoot.Spec.Provider.Workers[0].Minimum).To(Equal(int32(3)))
									Expect(shoot.Spec.Provider.Workers[0].Maximum).To(Equal(int32(3)))
								}

								commonShootValidation(gc, apiServer, shoot, flavor)
							})

							It("should set the high availability configuration to zone", func() {
								gc, apiServer := initGardenerHandlerTest(apiServerType, flavor, "testdata", "conversion", "apiserver-08.yaml")
								if strings.HasSuffix(flavor, "aws") {
									// aws doesn't have the 'europe-west3' region that is hardcoded in the apiserver config
									apiServer.Spec.GardenerConfig.Region = "eu-west-1"
								}
								_, gcfg, err := gc.LandscapeConfiguration(flavor)
								Expect(err).ToNot(HaveOccurred())
								shoot := &gardenv1beta1.Shoot{}
								Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())

								Expect(shoot.Namespace).To(Equal(gcfg.ProjectNamespace))

								Expect(shoot.Spec.ControlPlane).ToNot(BeNil())
								Expect(shoot.Spec.ControlPlane.HighAvailability).ToNot(BeNil())
								Expect(shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type).To(
									Equal(gardenv1beta1.FailureToleranceType(openmcpv1alpha1.HighAvailabilityFailureToleranceZone)))

								if apiServerType == openmcpv1alpha1.GardenerDedicated {
									Expect(shoot.Spec.Provider.Workers).To(HaveLen(1))
									Expect(shoot.Spec.Provider.Workers[0].Zones).To(HaveLen(3))
									Expect(shoot.Spec.Provider.Workers[0].Minimum).To(Equal(int32(3)))
									Expect(shoot.Spec.Provider.Workers[0].Maximum).To(Equal(int32(3)))
								}

								commonShootValidation(gc, apiServer, shoot, flavor)
							})

							It("should convert the EncryptionConfig configuration to the Shoot spec", func() {
								gc, apiServer := initGardenerHandlerTest(apiServerType, flavor, "testdata", "conversion", "apiserver-06.yaml")
								_, gcfg, err := gc.LandscapeConfiguration(flavor)
								Expect(err).ToNot(HaveOccurred())

								shoot := &gardenv1beta1.Shoot{}
								Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())

								Expect(shoot.Namespace).To(Equal(gcfg.ProjectNamespace))
								Expect(shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig.Resources).To(ConsistOf("configmaps", "statefulsets.apps", "flunders.example.com"))

								commonShootValidation(gc, apiServer, shoot, flavor)

								// removing the EncryptionConfig from the APIServer should remove it from the Shoot
								apiServer.Spec.GardenerConfig.EncryptionConfig = nil
								Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())
								Expect(shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig).To(BeNil())

								commonShootValidation(gc, apiServer, shoot, flavor)
							})

						})

					}

					Context(fmt.Sprintf("Tests specific to the '%s' APIServerType", openmcpv1alpha1.GardenerDedicated), func() {

						It("controlplane zone must not be overwritten and always part of any worker's zones", func() {
							gc, apiServer := initGardenerHandlerTest(openmcpv1alpha1.GardenerDedicated, flavor, "testdata", "conversion", "apiserver-01.yaml")
							shoot := &gardenv1beta1.Shoot{}
							Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())

							existingZone := "asia-south1-a"
							cpc := map[string]interface{}{}
							Expect(shoot.Spec.Provider.ControlPlaneConfig).ToNot(BeNil())
							Expect(yaml.Unmarshal(shoot.Spec.Provider.ControlPlaneConfig.Raw, &cpc)).To(Succeed())
							_, ok := cpc["zone"]
							if ok {
								// exchange zone to simulate an existing shoot with a different controlplane zone
								cpc["zone"] = existingZone
								cpcRaw, err := yaml.Marshal(cpc)
								Expect(err).ToNot(HaveOccurred())
								shoot.Spec.Provider.ControlPlaneConfig.Raw = cpcRaw
								// also set the worker's zones accordingly
								Expect(shoot.Spec.Provider.Workers).ToNot(BeEmpty())
								Expect(shoot.Spec.Provider.Workers[0].Zones).ToNot(BeEmpty())
								shoot.Spec.Provider.Workers[0].Zones[0] = existingZone

								Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())
								Expect(shoot.Spec.Provider.ControlPlaneConfig).ToNot(BeNil())
								Expect(yaml.Unmarshal(shoot.Spec.Provider.ControlPlaneConfig.Raw, &cpc)).To(Succeed())
								Expect(cpc).To(HaveKeyWithValue("zone", existingZone), "controlplane zone must not be changed")
								Expect(shoot.Spec.Provider.Workers).ToNot(BeEmpty())
								Expect(shoot.Spec.Provider.Workers[0].Zones).ToNot(BeEmpty())
								Expect(shoot.Spec.Provider.Workers[0].Zones).To(ContainElement(existingZone), "controlplane zone must be part of the worker's zones")
							}
							// nothing to do if the controlplane zone is not set
						})

					})

				})

			}

		})

	}

	Context("Multi-Config-Specific Tests", func() {

		for _, apiServerType := range []openmcpv1alpha1.APIServerType{openmcpv1alpha1.Gardener, openmcpv1alpha1.GardenerDedicated} {

			Context(fmt.Sprintf("Type: %s", string(apiServerType)), func() {

				It("should use a non-default configuration (default landscape), if set in shoot", func() {
					flavor := "default/aws"
					gc, apiServer := initGardenerHandlerTestMulti(apiServerType, flavor, "testdata", "conversion", "apiserver-03.yaml")
					_, gcfg, err := gc.LandscapeConfiguration(flavor)
					Expect(err).ToNot(HaveOccurred())
					shoot := &gardenv1beta1.Shoot{}
					Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())

					Expect(shoot.Namespace).To(Equal(gcfg.ProjectNamespace))
					Expect(shoot.Spec.Region).To(Equal(gcfg.DefaultRegion))
					Expect(shoot.Annotations).To(HaveKeyWithValue("test.openmcp.cloud/config", fmt.Sprintf("multi/%s", flavor)))
					commonShootValidation(gc, apiServer, shoot, flavor)
				})

				It("should use a non-default configuration (different landscape), if set in shoot", func() {
					flavor := "extra/foo"
					gc, apiServer := initGardenerHandlerTestMulti(apiServerType, flavor, "testdata", "conversion", "apiserver-03.yaml")
					_, gcfg, err := gc.LandscapeConfiguration(flavor)
					Expect(err).ToNot(HaveOccurred())
					shoot := &gardenv1beta1.Shoot{}
					Expect(gc.Shoot_v1beta1_from_APIServer_v1alpha1(env.Ctx, apiServer, shoot)).To(Succeed())

					Expect(shoot.Namespace).To(Equal(gcfg.ProjectNamespace))
					Expect(shoot.Spec.Region).To(Equal(gcfg.DefaultRegion))
					Expect(shoot.Annotations).To(HaveKeyWithValue("test.openmcp.cloud/config", fmt.Sprintf("multi/%s", flavor)))
					commonShootValidation(gc, apiServer, shoot, flavor)
				})

			})

		}

	})

})

// commonShootValidation is a helper function to test the parts of the shoot spec which are independent of the configuration.
// This is meant to avoid code duplication in the tests.
func commonShootValidation(gc *gardener.GardenerConnector, as *openmcpv1alpha1.APIServer, shoot *gardenv1beta1.Shoot, lc string) {
	_, gcfg, err := gc.LandscapeConfiguration(lc)
	Expect(err).ToNot(HaveOccurred())

	// metadata
	Expect(shoot.Annotations).To(HaveKey("shoot.gardener.cloud/cleanup-extended-apis-finalize-grace-period-seconds"))
	for k, v := range gcfg.ShootTemplate.Annotations {
		Expect(shoot.Annotations).To(HaveKeyWithValue(k, v))
	}
	Expect(shoot.Labels).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName, as.Name))
	Expect(shoot.Labels).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace, as.Namespace))
	if project, ok := as.Labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelProject]; ok {
		Expect(shoot.Labels).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelProject, project))
	}
	if workspace, ok := as.Labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelWorkspace]; ok {
		Expect(shoot.Labels).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelWorkspace, workspace))
	}
	for k, v := range gcfg.ShootTemplate.Labels {
		Expect(shoot.Labels).To(HaveKeyWithValue(k, v))
	}

	// spec.hibernation
	Expect(shoot.Spec.Hibernation.Enabled).To(PointTo(BeFalse()))

	// spec.cloudprofilename
	Expect(shoot.Spec.CloudProfile.Name).To(Equal(gcfg.CloudProfile))

	// spec.extensions
	Expect(shoot.Spec.Extensions).To(ContainElement(MatchFields(IgnoreExtras, Fields{
		"Type": Equal("shoot-oidc-service"),
	})))

	// spec.provider
	Expect(shoot.Spec.Provider.Type).To(Equal(gcfg.ProviderType))
	switch as.Spec.Type {
	case openmcpv1alpha1.Gardener:
		Expect(shoot.Spec.Provider.Workers).To(BeEmpty())
	case openmcpv1alpha1.GardenerDedicated:
		Expect(shoot.Spec.Networking.Type).To(Equal(gcfg.ShootTemplate.Spec.Networking.Type))
		Expect(shoot.Spec.Networking.Nodes).To(Equal(gcfg.ShootTemplate.Spec.Networking.Nodes))
		Expect(shoot.Spec.SecretBindingName).To(Equal(gcfg.ShootTemplate.Spec.SecretBindingName))
		Expect(shoot.Spec.Provider.Workers).ToNot(BeEmpty())

		switch shoot.Spec.Provider.Type {
		case "gcp":
			cpc := map[string]interface{}{}
			Expect(yaml.Unmarshal(shoot.Spec.Provider.ControlPlaneConfig.Raw, &cpc)).To(Succeed())
			Expect(cpc).To(HaveKeyWithValue("apiVersion", "gcp.provider.extensions.gardener.cloud/v1alpha1"))
			Expect(cpc).To(HaveKey("zone"))
			cpZone, ok := cpc["zone"].(string)
			Expect(ok).To(BeTrue(), "spec.provider.controlPlaneConfig.zone is not a string")
			Expect(gcfg.ValidRegions[shoot.Spec.Region].Zones).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name": Equal(cpZone),
			})), "spec.provider.controlPlaneConfig.zone is not a valid zone")
			foundCPC := false
			if len(shoot.Spec.Provider.Workers) > 0 {
				for _, w := range shoot.Spec.Provider.Workers {
					for _, z := range w.Zones {
						if z == cpZone {
							foundCPC = true
							break
						}
					}
					if foundCPC {
						break
					}
				}
				Expect(foundCPC).To(BeTrue(), "control plane zone is not contained in any worker's zones")
			}
		case "aws":
			cpc := map[string]interface{}{}
			Expect(yaml.Unmarshal(shoot.Spec.Provider.ControlPlaneConfig.Raw, &cpc)).To(Succeed())
			Expect(cpc).To(HaveKeyWithValue("apiVersion", "aws.provider.extensions.gardener.cloud/v1alpha1"))
			awsInfraCfg := &gardenawsv1alpha1.InfrastructureConfig{}
			Expect(yaml.Unmarshal(shoot.Spec.Provider.InfrastructureConfig.Raw, awsInfraCfg)).To(Succeed())
			Expect(awsInfraCfg.Networks.Zones).To(HaveLen(len(gcfg.ValidRegions[shoot.Spec.Region].Zones)))
			Expect(awsInfraCfg.Networks.VPC.CIDR).ToNot(BeNil())
			_, vpc, err := net.ParseCIDR(*awsInfraCfg.Networks.VPC.CIDR)
			Expect(err).ToNot(HaveOccurred())
			// verify that all zone CIDRs are within the VPC CIDR, but completely disjunct from each other
			cidrs := make([]*net.IPNet, 0, len(awsInfraCfg.Networks.Zones)*3)
			for _, z := range awsInfraCfg.Networks.Zones {
				_, workers, err := net.ParseCIDR(z.Workers)
				Expect(err).ToNot(HaveOccurred())
				_, public, err := net.ParseCIDR(z.Public)
				Expect(err).ToNot(HaveOccurred())
				_, internal, err := net.ParseCIDR(z.Internal)
				Expect(err).ToNot(HaveOccurred())
				cidrs = append(cidrs, workers, public, internal)
			}
			Expect(cidr.VerifyNoOverlap(cidrs, vpc)).To(Succeed())
		}
	}

	// spec.kubernetes
	Expect(shoot.Spec.Kubernetes.KubeAPIServer.RuntimeConfig).To(HaveKeyWithValue("apps/v1", true))
	Expect(shoot.Spec.Kubernetes.KubeAPIServer.RuntimeConfig).To(HaveKeyWithValue("batch/v1", true))
}
