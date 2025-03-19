package components_test

import (
	"reflect"

	"github.com/openmcp-project/mcp-operator/internal/components"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

var _ = Describe("CloudOrchestratorConverter", func() {
	Context("ConvertToResourceSpec", func() {
		It("should convert the spec", func() {
			conv := &components.CloudOrchestratorConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						CloudOrchestratorConfiguration: openmcpv1alpha1.CloudOrchestratorConfiguration{
							Crossplane: &openmcpv1alpha1.CrossplaneConfig{
								Version: "v1",
								Providers: []*openmcpv1alpha1.CrossplaneProviderConfig{
									{
										Name:    "foo",
										Version: "v2.1.0",
									},
								},
							},
							BTPServiceOperator:      nil,
							ExternalSecretsOperator: nil,
							Kyverno:                 nil,
							Flux:                    nil,
						},
					},
				},
			}

			authSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(authSpec).ToNot(BeNil())
			Expect(authSpec).To(BeAssignableToTypeOf(&openmcpv1alpha1.CloudOrchestratorSpec{}))

			coSpecT := authSpec.(*openmcpv1alpha1.CloudOrchestratorSpec)
			Expect(reflect.DeepEqual(coSpecT.Crossplane, mcp.Spec.Components.Crossplane)).To(BeTrue())
		})

		It("should convert the spec with default values", func() {
			conv := &components.CloudOrchestratorConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			}

			coSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(coSpec).ToNot(BeNil())

			coSpecT := coSpec.(*openmcpv1alpha1.CloudOrchestratorSpec)
			Expect(coSpecT.Crossplane).To(BeNil())
			Expect(coSpecT.BTPServiceOperator).To(BeNil())
			Expect(coSpecT.ExternalSecretsOperator).To(BeNil())
			Expect(coSpecT.Kyverno).To(BeNil())
			Expect(coSpecT.Flux).To(BeNil())
		})
	})

	Context("InjectStatus", func() {
		It("should inject the status", func() {
			conv := &components.CloudOrchestratorConverter{}
			mcpStatus := &openmcpv1alpha1.ManagedControlPlaneStatus{}
			status := &openmcpv1alpha1.ExternalCloudOrchestratorStatus{}

			err := conv.InjectStatus(status, mcpStatus)
			Expect(err).ToNot(HaveOccurred())
			Expect(mcpStatus.Components.CloudOrchestrator).To(Equal(status))
		})

		It("should fail to inject the status", func() {
			conv := &components.CloudOrchestratorConverter{}
			mcpStatus := &openmcpv1alpha1.ManagedControlPlaneStatus{}
			unknownStatus := struct {
				Foo string
			}{
				Foo: "bar",
			}

			err := conv.InjectStatus(unknownStatus, mcpStatus)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("IsConfigured", func() {
		It("should return true if the crossplane configured", func() {
			conv := &components.CloudOrchestratorConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						CloudOrchestratorConfiguration: openmcpv1alpha1.CloudOrchestratorConfiguration{
							Crossplane: &openmcpv1alpha1.CrossplaneConfig{},
						},
					},
				},
			}

			Expect(conv.IsConfigured(mcp)).To(BeTrue())
		})

		It("should return true if the BTPServiceOperator configured", func() {
			conv := &components.CloudOrchestratorConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						CloudOrchestratorConfiguration: openmcpv1alpha1.CloudOrchestratorConfiguration{
							BTPServiceOperator: &openmcpv1alpha1.BTPServiceOperatorConfig{},
						},
					},
				},
			}

			Expect(conv.IsConfigured(mcp)).To(BeTrue())
		})

		It("should return true if the ExternalSecretsOperator configured", func() {
			conv := &components.CloudOrchestratorConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						CloudOrchestratorConfiguration: openmcpv1alpha1.CloudOrchestratorConfiguration{
							ExternalSecretsOperator: &openmcpv1alpha1.ExternalSecretsOperatorConfig{},
						},
					},
				},
			}

			Expect(conv.IsConfigured(mcp)).To(BeTrue())
		})

		It("should return true if the Kyverno configured", func() {
			conv := &components.CloudOrchestratorConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						CloudOrchestratorConfiguration: openmcpv1alpha1.CloudOrchestratorConfiguration{
							Kyverno: &openmcpv1alpha1.KyvernoConfig{},
						},
					},
				},
			}

			Expect(conv.IsConfigured(mcp)).To(BeTrue())
		})

		It("should return true if the Flux configured", func() {
			conv := &components.CloudOrchestratorConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						CloudOrchestratorConfiguration: openmcpv1alpha1.CloudOrchestratorConfiguration{
							Flux: &openmcpv1alpha1.FluxConfig{},
						},
					},
				},
			}

			Expect(conv.IsConfigured(mcp)).To(BeTrue())
		})

		It("should return false no component is configured", func() {
			conv := &components.CloudOrchestratorConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Components: openmcpv1alpha1.ManagedControlPlaneComponents{
						CloudOrchestratorConfiguration: openmcpv1alpha1.CloudOrchestratorConfiguration{},
					},
				},
			}

			Expect(conv.IsConfigured(mcp)).To(BeFalse())
		})
	})
})
