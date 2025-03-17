package components_test

import (
	"reflect"

	"github.tools.sap/CoLa/mcp-operator/internal/components"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

var _ = Describe("AuthenticationConverter", func() {
	Context("ConvertToResourceSpec", func() {
		It("should convert the spec", func() {
			conv := &components.AuthenticationConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Authentication: &openmcpv1alpha1.AuthenticationConfiguration{
						EnableSystemIdentityProvider: ptr.To(false),
						IdentityProviders: []openmcpv1alpha1.IdentityProvider{
							{
								Name:          "test",
								IssuerURL:     "https://test",
								ClientID:      "aaa-bbb-ccc",
								GroupsClaim:   "grps1",
								UsernameClaim: "usr1",
							},
						},
					},
				},
			}

			authSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(authSpec).ToNot(BeNil())
			Expect(authSpec).To(BeAssignableToTypeOf(&openmcpv1alpha1.AuthenticationSpec{}))

			authSpecT := authSpec.(*openmcpv1alpha1.AuthenticationSpec)
			Expect(reflect.DeepEqual(authSpecT.AuthenticationConfiguration, *mcp.Spec.Authentication)).To(BeTrue())
		})

		It("should convert the spec with default values", func() {
			conv := &components.AuthenticationConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			}

			authSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(authSpec).ToNot(BeNil())
			Expect(authSpec).To(BeAssignableToTypeOf(&openmcpv1alpha1.AuthenticationSpec{}))

			authSpecT := authSpec.(*openmcpv1alpha1.AuthenticationSpec)
			Expect(authSpecT.AuthenticationConfiguration.EnableSystemIdentityProvider).To(Equal(ptr.To(true)))
			Expect(authSpecT.AuthenticationConfiguration.IdentityProviders).To(BeEmpty())
		})

		It("should not covert an invalid spec", func() {
			conv := &components.AuthenticationConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Authentication: &openmcpv1alpha1.AuthenticationConfiguration{
						EnableSystemIdentityProvider: ptr.To(false),
						IdentityProviders: []openmcpv1alpha1.IdentityProvider{
							{
								Name:          "test",
								IssuerURL:     "https://test",
								ClientID:      "aaa-bbb-ccc",
								GroupsClaim:   "grps1",
								UsernameClaim: "usr1",
							},
							{
								Name:          "test",
								IssuerURL:     "https://test1",
								ClientID:      "aaa-bbb-ccc",
								GroupsClaim:   "grps2",
								UsernameClaim: "usr2",
							},
						},
					},
				},
			}

			authSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).To(HaveOccurred())
			Expect(authSpec).To(BeNil())
		})
	})

	Context("InjectStatus", func() {
		It("should inject the status", func() {
			conv := &components.AuthenticationConverter{}
			status := &openmcpv1alpha1.ExternalAuthenticationStatus{
				UserAccess: &openmcpv1alpha1.SecretReference{
					NamespacedObjectReference: openmcpv1alpha1.NamespacedObjectReference{
						Name:      "test",
						Namespace: "test",
					},
					Key: "access",
				},
			}

			mcpStatus := &openmcpv1alpha1.ManagedControlPlaneStatus{}

			err := conv.InjectStatus(status, mcpStatus)
			Expect(err).ToNot(HaveOccurred())
			Expect(mcpStatus.Components.Authentication).ToNot(BeNil())
			Expect(mcpStatus.Components.Authentication).To(Equal(status))
		})

		It("should not inject an incompatible status", func() {
			conv := &components.AuthenticationConverter{}
			unknownStatus := struct {
				Foo string
			}{
				Foo: "bar",
			}

			mcpStatus := &openmcpv1alpha1.ManagedControlPlaneStatus{}
			err := conv.InjectStatus(unknownStatus, mcpStatus)
			Expect(err).To(HaveOccurred())
			Expect(mcpStatus.Components.Authentication).To(BeNil())
		})
	})

	Context("IsConfigured", func() {
		It("should return true if the spec is configured", func() {
			conv := &components.AuthenticationConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Authentication: &openmcpv1alpha1.AuthenticationConfiguration{},
				},
			}

			Expect(conv.IsConfigured(mcp)).To(BeTrue())
		})

		It("should return false if the spec is not configured", func() {
			conv := &components.AuthenticationConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{}

			Expect(conv.IsConfigured(mcp)).To(BeFalse())
		})
	})
})
