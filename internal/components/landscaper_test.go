package components_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.tools.sap/CoLa/mcp-operator/internal/components"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

var _ = Describe("LandscaperConverter", func() {
	Context("ConvertToResourceSpec", func() {
		It("should convert the spec", func() {
			conv := &components.LandscaperConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						Landscaper: &openmcpv1alpha1.LandscaperConfiguration{
							Deployers: []string{
								"manifest",
								"helm",
							},
						},
					},
				},
			}

			lsSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(lsSpec).ToNot(BeNil())
			Expect(lsSpec).To(BeAssignableToTypeOf(&openmcpv1alpha1.LandscaperSpec{}))

			lsSpecT := lsSpec.(*openmcpv1alpha1.LandscaperSpec)
			Expect(lsSpecT.Deployers).To(Equal(mcp.Spec.Components.Landscaper.Deployers))
		})

		It("should convert the spec with default values", func() {
			conv := &components.LandscaperConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						Landscaper: &openmcpv1alpha1.LandscaperConfiguration{},
					},
				},
			}

			lsSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(lsSpec).ToNot(BeNil())
			Expect(lsSpec).To(BeAssignableToTypeOf(&openmcpv1alpha1.LandscaperSpec{}))

			lsSpecT := lsSpec.(*openmcpv1alpha1.LandscaperSpec)
			Expect(lsSpecT.Deployers).To(ConsistOf("helm", "manifest", "container"))
		})

		It("should return an error if the spec is not configured", func() {
			conv := &components.LandscaperConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			}

			lsSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).To(HaveOccurred())
			Expect(lsSpec).To(BeNil())
		})
	})

	Context("InjectStatus", func() {
		It("should inject the status", func() {
			conv := &components.LandscaperConverter{}
			mcpStatus := &openmcpv1alpha1.ManagedControlPlaneStatus{}
			status := &openmcpv1alpha1.ExternalLandscaperStatus{}

			err := conv.InjectStatus(status, mcpStatus)
			Expect(err).ToNot(HaveOccurred())
			Expect(mcpStatus.Components.Landscaper).To(Equal(status))
		})

		It("should not inject an incompatible status", func() {
			conv := &components.LandscaperConverter{}
			unknownStatus := struct {
				Foo string
			}{
				Foo: "bar",
			}

			mcpStatus := &openmcpv1alpha1.ManagedControlPlaneStatus{}
			err := conv.InjectStatus(unknownStatus, mcpStatus)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("IsConfigured", func() {
		It("should return true if the spec is configured", func() {
			conv := &components.LandscaperConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						Landscaper: &openmcpv1alpha1.LandscaperConfiguration{},
					},
				},
			}

			Expect(conv.IsConfigured(mcp)).To(BeTrue())
		})

		It("should return false if the spec is not configured", func() {
			conv := &components.LandscaperConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{}

			Expect(conv.IsConfigured(mcp)).To(BeFalse())
		})
	})
})
