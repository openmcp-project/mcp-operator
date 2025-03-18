package config_test

import (
	"context"
	"fmt"
	"path"
	"testing"

	apiserverconfig "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/config"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	openmcptesting "github.com/openmcp-project/controller-utils/pkg/testing"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	testutils "github.com/openmcp-project/mcp-operator/test/utils"
)

const (
	gardenCluster  = "garden"
	gardenCluster2 = "garden2"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "APIServer Config Test Suite")
}

var _ = Describe("APIServer Config", func() {

	Context("Config Loading", func() {

		It("should load and complete a valid config", func() {
			cfgFile := path.Join("testdata", "config_valid.yaml")
			cfg, err := apiserverconfig.LoadConfig(cfgFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())

			env := openmcptesting.NewEnvironmentBuilder().WithFakeClient(testutils.Scheme).WithInitObjectPath("testdata", "garden_cluster").Build()
			cfg.GardenerConfig.InjectGardenClusterClient("", env.Client())

			cc, err := cfg.Complete(context.TODO())
			Expect(err).ToNot(HaveOccurred())
			Expect(cc).ToNot(BeNil())
			Expect(cc.ServiceAccountNamespace).To(Equal(openmcpv1alpha1.SystemNamespace))
			Expect(cc.AdminServiceAccountName).To(Equal("admin"))
		})

		It("should fail to load an incorrect config", func() {
			cfgFile := path.Join("testdata", "config_invalid-1.yaml")
			_, err := apiserverconfig.LoadConfig(cfgFile)
			Expect(err).To(HaveOccurred())
		})

		It("should fail to load a non-existing config", func() {
			cfgFile := path.Join("testdata", "config_non_existing.yaml")
			_, err := apiserverconfig.LoadConfig(cfgFile)
			Expect(err).To(HaveOccurred())
		})

		Context("Gardener Config Loading", func() {

			for _, configMode := range []string{"single", "multi"} {
				var affix string
				var injectKubeconfigs func(cfg *apiserverconfig.MultiGardenerConfiguration, env *openmcptesting.ComplexEnvironment)
				switch configMode {
				case "multi":
					affix = "multi_"
					injectKubeconfigs = func(cfg *apiserverconfig.MultiGardenerConfiguration, env *openmcptesting.ComplexEnvironment) {
						cfg.InjectGardenClusterClient("default", env.Client(gardenCluster))
						cfg.InjectGardenClusterClient("extra", env.Client(gardenCluster2))
					}
				default:
					affix = ""
					injectKubeconfigs = func(cfg *apiserverconfig.MultiGardenerConfiguration, env *openmcptesting.ComplexEnvironment) {
						cfg.InjectGardenClusterClient("", env.Client(gardenCluster))
					}
				}

				Context(fmt.Sprintf("Config Mode: %s", configMode), func() {

					It("should fail to complete a config if the default region is not in the configured regions", func() {
						cfgFile := path.Join("testdata", fmt.Sprintf("config_%sinvalid_region-1.yaml", affix))
						cfg, err := apiserverconfig.LoadConfig(cfgFile)
						Expect(err).ToNot(HaveOccurred())
						Expect(cfg).ToNot(BeNil())

						env := openmcptesting.NewComplexEnvironmentBuilder().WithFakeClient(gardenCluster, testutils.Scheme).WithInitObjectPath(gardenCluster, "testdata", "garden_cluster").WithFakeClient(gardenCluster2, testutils.Scheme).WithInitObjectPath(gardenCluster2, "testdata", "garden_cluster_2").Build()
						injectKubeconfigs(cfg.GardenerConfig, env)

						_, err = cfg.Complete(context.TODO())
						Expect(err).To(MatchError(ContainSubstring("default region")))
						if configMode == "multi" {
							Expect(err).To(MatchError(ContainSubstring("extra/foo")))
						}
					})

					It("should fail to complete a config if the default region is in the configured regions but not in the cloudprofile", func() {
						cfgFile := path.Join("testdata", fmt.Sprintf("config_%sinvalid_region-2.yaml", affix))
						cfg, err := apiserverconfig.LoadConfig(cfgFile)
						Expect(err).ToNot(HaveOccurred())
						Expect(cfg).ToNot(BeNil())

						env := openmcptesting.NewComplexEnvironmentBuilder().WithFakeClient(gardenCluster, testutils.Scheme).WithInitObjectPath(gardenCluster, "testdata", "garden_cluster").WithFakeClient(gardenCluster2, testutils.Scheme).WithInitObjectPath(gardenCluster2, "testdata", "garden_cluster_2").Build()
						injectKubeconfigs(cfg.GardenerConfig, env)

						_, err = cfg.Complete(context.TODO())
						Expect(err).To(MatchError(ContainSubstring("default region")))
						if configMode == "multi" {
							Expect(err).To(MatchError(ContainSubstring("extra/foo")))
						}
					})

				})
			}

		})

	})

	Context("Config Validation", func() {

		It("should correctly validate a valid config", func() {
			cfgFile := path.Join("testdata", "config_valid.yaml")
			cfg, err := apiserverconfig.LoadConfig(cfgFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(apiserverconfig.Validate(cfg)).To(Succeed())
		})

		It("should correctly validate a valid config (Gardener multi)", func() {
			cfgFile := path.Join("testdata", "config_multi_valid.yaml")
			cfg, err := apiserverconfig.LoadConfig(cfgFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(apiserverconfig.Validate(cfg)).To(Succeed())
		})

		Context("Gardener Config Validation", func() {

			It("should detect if the default landscape/configuration is missing for multi mode", func() {
				cfgFile := path.Join("testdata", "config_multi_invalid_missing_default.yaml")
				cfg, err := apiserverconfig.LoadConfig(cfgFile)
				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())
				err = apiserverconfig.Validate(cfg)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("default"))
			})

			for _, configMode := range []string{"single", "multi"} {
				affix := ""
				switch configMode {
				case "multi":
					affix = "multi_"
				}

				Context(fmt.Sprintf("Config Mode: %s", configMode), func() {

					It("should detect if the regions are missing", func() {
						cfgFile := path.Join("testdata", fmt.Sprintf("config_%sinvalid-2.yaml", affix))
						cfg, err := apiserverconfig.LoadConfig(cfgFile)
						Expect(err).ToNot(HaveOccurred())
						Expect(cfg).ToNot(BeNil())
						err = apiserverconfig.Validate(cfg)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("regions"))
					})

					It("should detect if the cloudprofile is missing", func() {
						cfgFile := path.Join("testdata", fmt.Sprintf("config_%sinvalid-3.yaml", affix))
						cfg, err := apiserverconfig.LoadConfig(cfgFile)
						Expect(err).ToNot(HaveOccurred())
						Expect(cfg).ToNot(BeNil())
						err = apiserverconfig.Validate(cfg)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("cloudProfile"))
					})

					It("should detect if the shoot template is missing", func() {
						cfgFile := path.Join("testdata", fmt.Sprintf("config_%sinvalid-4.yaml", affix))
						cfg, err := apiserverconfig.LoadConfig(cfgFile)
						Expect(err).ToNot(HaveOccurred())
						Expect(cfg).ToNot(BeNil())
						err = apiserverconfig.Validate(cfg)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("shootTemplate"))
					})

					It("should detect if the project is missing", func() {
						cfgFile := path.Join("testdata", fmt.Sprintf("config_%sinvalid-5.yaml", affix))
						cfg, err := apiserverconfig.LoadConfig(cfgFile)
						Expect(err).ToNot(HaveOccurred())
						Expect(cfg).ToNot(BeNil())
						err = apiserverconfig.Validate(cfg)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("project"))
					})

					It("should detect if the kubeconfig is missing", func() {
						cfgFile := path.Join("testdata", fmt.Sprintf("config_%sinvalid-6.yaml", affix))
						cfg, err := apiserverconfig.LoadConfig(cfgFile)
						Expect(err).ToNot(HaveOccurred())
						Expect(cfg).ToNot(BeNil())
						err = apiserverconfig.Validate(cfg)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("kubeconfig"))
					})

				})
			}

		})

	})

})
