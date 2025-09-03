package components_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openmcp-project/mcp-operator/internal/components"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

var _ = Describe("APIServerConverter", func() {
	Context("ConvertToResourceSpec", func() {
		It("should convert the spec", func() {
			conv := &components.APIServerConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						APIServer: &openmcpv1alpha1.APIServerConfiguration{
							Type: openmcpv1alpha1.Gardener,
							GardenerConfig: &openmcpv1alpha1.GardenerConfiguration{
								Region: "europe",
							},
						},
					},
				},
			}

			apiServerSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(apiServerSpec).ToNot(BeNil())
			Expect(apiServerSpec).To(BeAssignableToTypeOf(&openmcpv1alpha1.APIServerSpec{}))

			apiServerSpecT := apiServerSpec.(*openmcpv1alpha1.APIServerSpec)
			Expect(apiServerSpecT.Type).To(Equal(mcp.Spec.Components.APIServer.Type))
			Expect(apiServerSpecT.GardenerConfig).To(Equal(mcp.Spec.Components.APIServer.GardenerConfig))
		})

		It("should convert the spec with an internal configuration and common configuration", func() {
			conv := &components.APIServerConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						APIServer: &openmcpv1alpha1.APIServerConfiguration{
							Type: openmcpv1alpha1.Gardener,
							GardenerConfig: &openmcpv1alpha1.GardenerConfiguration{
								Region: "europe",
							},
						},
					},
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
					Components: openmcpv1alpha1.InternalConfigurationComponents{
						APIServer: &openmcpv1alpha1.APIServerInternalConfiguration{
							GardenerConfig: &openmcpv1alpha1.GardenerInternalConfiguration{
								ShootOverwrite: &openmcpv1alpha1.NamespacedObjectReference{
									Name:      "test",
									Namespace: "test",
								},
							},
						},
					},
				},
			}

			apiServerSpec, err := conv.ConvertToResourceSpec(mcp, icfg)
			Expect(err).ToNot(HaveOccurred())
			Expect(apiServerSpec).ToNot(BeNil())
			Expect(apiServerSpec).To(BeAssignableToTypeOf(&openmcpv1alpha1.APIServerSpec{}))

			apiServerSpecT := apiServerSpec.(*openmcpv1alpha1.APIServerSpec)
			Expect(apiServerSpecT.Type).To(Equal(mcp.Spec.Components.APIServer.Type))
			Expect(apiServerSpecT.GardenerConfig).To(Equal(mcp.Spec.Components.APIServer.GardenerConfig))
			Expect(apiServerSpecT.Internal.GardenerConfig.ShootOverwrite).To(Equal(icfg.Spec.Components.APIServer.GardenerConfig.ShootOverwrite))
			Expect(apiServerSpecT.DesiredRegion).To(Equal(mcp.Spec.DesiredRegion))
		})

		It("should return an error if the spec is not configured", func() {
			conv := &components.APIServerConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			}

			apiServerSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).To(HaveOccurred())
			Expect(apiServerSpec).To(BeNil())
		})
	})

	Context("InjectStatus", func() {
		It("should inject the status", func() {
			conv := &components.APIServerConverter{}
			status := openmcpv1alpha1.ExternalAPIServerStatus{}

			mcpStatus := &openmcpv1alpha1.ManagedControlPlaneStatus{}

			err := conv.InjectStatus(status, mcpStatus)
			Expect(err).ToNot(HaveOccurred())
			Expect(mcpStatus.Components.APIServer).ToNot(BeNil())
			Expect(*mcpStatus.Components.APIServer).To(Equal(status))
		})

		It("should not inject an incompatible status", func() {
			conv := &components.APIServerConverter{}
			unknownStatus := struct {
				Foo string
			}{
				Foo: "bar",
			}

			mcpStatus := &openmcpv1alpha1.ManagedControlPlaneStatus{}
			err := conv.InjectStatus(unknownStatus, mcpStatus)
			Expect(err).To(HaveOccurred())
			Expect(mcpStatus.Components.APIServer).To(BeNil())
		})
	})

	Context("IsConfigured", func() {
		It("should return true if the spec is configured", func() {
			conv := &components.APIServerConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						APIServer: &openmcpv1alpha1.APIServerConfiguration{
							Type: openmcpv1alpha1.Gardener,
							GardenerConfig: &openmcpv1alpha1.GardenerConfiguration{
								Region: "europe",
							},
						},
					},
				},
			}

			Expect(conv.IsConfigured(mcp)).To(BeTrue())
		})

		It("should return false if the spec is not configured", func() {
			conv := &components.APIServerConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{}

			Expect(conv.IsConfigured(mcp)).To(BeFalse())
		})
	})
})
