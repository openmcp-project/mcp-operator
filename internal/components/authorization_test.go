package components_test

import (
	"reflect"

	"github.com/openmcp-project/mcp-operator/internal/components"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

var _ = Describe("AuthorizationConverter", func() {
	Context("ConvertToResourceSpec", func() {
		It("should convert the spec", func() {
			conv := &components.AuthorizationConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Authorization: &openmcpv1alpha1.AuthorizationConfiguration{
						RoleBindings: []openmcpv1alpha1.RoleBinding{
							{

								Role: "admin",
								Subjects: []openmcpv1alpha1.Subject{
									{
										APIGroup: rbacv1.GroupName,
										Kind:     "User",
										Name:     "admin",
									},
								},
							},
						},
					},
				},
			}

			authSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(authSpec).ToNot(BeNil())
			Expect(authSpec).To(BeAssignableToTypeOf(&openmcpv1alpha1.AuthorizationSpec{}))

			authSpecT := authSpec.(*openmcpv1alpha1.AuthorizationSpec)
			Expect(reflect.DeepEqual(authSpecT.AuthorizationConfiguration, *mcp.Spec.Authorization)).To(BeTrue())
		})

		It("should return an error if the spec is not configured", func() {
			conv := &components.AuthorizationConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			}

			authSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).To(HaveOccurred())
			Expect(authSpec).To(BeNil())
		})

		It("should convert the spec with default values", func() {
			conv := &components.AuthorizationConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Authorization: &openmcpv1alpha1.AuthorizationConfiguration{
						RoleBindings: []openmcpv1alpha1.RoleBinding{
							{
								Role: "admin",
								Subjects: []openmcpv1alpha1.Subject{
									{
										Kind: "User",
										Name: "test",
									},
									{
										Kind: "Group",
										Name: "admin",
									},
								},
							},
							{
								Role: "view",
								Subjects: []openmcpv1alpha1.Subject{
									{
										Kind:      "ServiceAccount",
										Name:      "automate",
										Namespace: "default",
									},
								},
							},
						},
					},
				},
			}

			authSpec, err := conv.ConvertToResourceSpec(mcp, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(authSpec).ToNot(BeNil())
			Expect(authSpec).To(BeAssignableToTypeOf(&openmcpv1alpha1.AuthorizationSpec{}))

			authSpecT := authSpec.(*openmcpv1alpha1.AuthorizationSpec)
			Expect(authSpecT.RoleBindings[0].Subjects[0].APIGroup).To(Equal(rbacv1.GroupName))
			Expect(authSpecT.RoleBindings[0].Subjects[1].APIGroup).To(Equal(rbacv1.GroupName))

		})

		It("should not covert an invalid spec", func() {
			conv := &components.AuthorizationConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Authorization: &openmcpv1alpha1.AuthorizationConfiguration{
						RoleBindings: []openmcpv1alpha1.RoleBinding{
							{
								Role: "invalid",
								Subjects: []openmcpv1alpha1.Subject{
									{
										Kind: "invalid",
									},
								},
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
			conv := &components.AuthorizationConverter{}
			mcpStatus := &openmcpv1alpha1.ManagedControlPlaneStatus{}
			status := &openmcpv1alpha1.ExternalAuthorizationStatus{}

			err := conv.InjectStatus(status, mcpStatus)
			Expect(err).ToNot(HaveOccurred())
			Expect(mcpStatus.Components.Authorization).To(Equal(status))
		})

		It("should fail to inject the status", func() {
			conv := &components.AuthorizationConverter{}
			mcpStatus := &openmcpv1alpha1.ManagedControlPlaneStatus{}
			unknownStatus := struct {
				Foo string
			}{
				Foo: "bar",
			}

			err := conv.InjectStatus(unknownStatus, mcpStatus)
			Expect(err).To(HaveOccurred())
			Expect(mcpStatus.Components.Authorization).To(BeNil())
		})
	})

	Context("IsConfigured", func() {
		It("should return true if the spec is configured", func() {
			conv := &components.AuthorizationConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{
				Spec: openmcpv1alpha1.ManagedControlPlaneSpec{
					Authorization: &openmcpv1alpha1.AuthorizationConfiguration{},
				},
			}

			Expect(conv.IsConfigured(mcp)).To(BeTrue())
		})

		It("should return false if the spec is not configured", func() {
			conv := &components.AuthorizationConverter{}
			mcp := &openmcpv1alpha1.ManagedControlPlane{}

			Expect(conv.IsConfigured(mcp)).To(BeFalse())
		})
	})
})
