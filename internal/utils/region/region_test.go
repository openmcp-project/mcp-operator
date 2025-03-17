package region

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

var _ = Describe("Regions", func() {

	It("should find one closest region", func() {
		availableRegions := []string{"europe-west1", "us-east1", "us-west1"}
		origin := openmcpv1alpha1.RegionSpecification{
			Name:      openmcpv1alpha1.ASIA,
			Direction: openmcpv1alpha1.CENTRAL,
		}
		regions, err := GetClosestRegions(origin, GCPMapper(), availableRegions, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(regions).To(HaveLen(1))
		Expect(regions[0]).To(Equal(availableRegions[2]))
	})

	It("should find the closest regions", func() {
		availableRegions := []string{"europe-west1", "us-east1", "us-west1"}
		origin := openmcpv1alpha1.RegionSpecification{
			Name:      openmcpv1alpha1.NORTHAMERICA,
			Direction: openmcpv1alpha1.NORTH,
		}
		regions, err := GetClosestRegions(origin, GCPMapper(), availableRegions, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(regions).To(HaveLen(2))
		Expect(regions).To(ContainElements(availableRegions[1], availableRegions[2]))
	})
})
