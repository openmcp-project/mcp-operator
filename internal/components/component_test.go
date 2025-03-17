package components_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.tools.sap/CoLa/mcp-operator/internal/components"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

var _ = Describe("Components", func() {
	Context("GetCommonConfig", func() {
		It("should convert the spec", func() {
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					CommonConfig: &openmcpv1alpha1.CommonConfig{
						DesiredRegion: &openmcpv1alpha1.RegionSpecification{
							Name:      "europe",
							Direction: "west",
						},
					},
				},
			}
			icfg := &openmcpv1alpha1.InternalConfiguration{
				Spec: openmcpv1alpha1.InternalConfigurationSpec{
					InternalCommonConfig: &openmcpv1alpha1.InternalCommonConfig{},
				},
			}
			commonCfg, iCommonConfig := components.GetCommonConfig(mcp, icfg)
			Expect(commonCfg).ToNot(BeNil())
			Expect(iCommonConfig).ToNot(BeNil())
			Expect(commonCfg.DesiredRegion).To(Equal(mcp.Spec.CommonConfig.DesiredRegion))
			Expect(iCommonConfig).To(Equal(icfg.Spec.InternalCommonConfig))
		})

		It("should return nil if the spec is not configured", func() {
			mcp := &openmcpv1alpha1.ManagedControlPlane{}
			icfg := &openmcpv1alpha1.InternalConfiguration{}
			commonCfg, iCommonConfig := components.GetCommonConfig(mcp, icfg)
			Expect(commonCfg).To(BeNil())
			Expect(iCommonConfig).To(BeNil())
		})
	})
})
