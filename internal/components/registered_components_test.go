package components_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	components "github.tools.sap/CoLa/mcp-operator/internal/components"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

var _ = Describe("ComponentHandler", func() {
	Context("Registry", func() {
		It("should return the known components", func() {
			components := components.Registry.GetKnownComponents()
			Expect(components).ToNot(BeNil())
			Expect(components).To(HaveLen(5))
			Expect(components).To(HaveKey(openmcpv1alpha1.AuthenticationComponent))
			Expect(components).To(HaveKey(openmcpv1alpha1.AuthorizationComponent))
			Expect(components).To(HaveKey(openmcpv1alpha1.APIServerComponent))
			Expect(components).To(HaveKey(openmcpv1alpha1.CloudOrchestratorComponent))
			Expect(components).To(HaveKey(openmcpv1alpha1.LandscaperComponent))
		})

		It("should return true for known components", func() {
			Expect(components.Registry.Has(openmcpv1alpha1.AuthenticationComponent)).To(BeTrue())
			Expect(components.Registry.Has(openmcpv1alpha1.AuthorizationComponent)).To(BeTrue())
			Expect(components.Registry.Has(openmcpv1alpha1.APIServerComponent)).To(BeTrue())
			Expect(components.Registry.Has(openmcpv1alpha1.CloudOrchestratorComponent)).To(BeTrue())
			Expect(components.Registry.Has(openmcpv1alpha1.LandscaperComponent)).To(BeTrue())
		})

		It("should return false for unknown components", func() {
			Expect(components.Registry.Has("unknown")).To(BeFalse())
		})

		It("should return the scheme", func() {
			scheme := components.Registry.Scheme()
			Expect(scheme).ToNot(BeNil())
		})
	})

	Context("Authentication", func() {
		var (
			converter components.ComponentConverter
			handler   *components.ComponentHandler
		)

		BeforeEach(func() {
			handler = components.Registry.GetComponent(openmcpv1alpha1.AuthenticationComponent)
			Expect(handler).ToNot(BeNil())
			converter = handler.Converter()
			Expect(converter).ToNot(BeNil())

		})

		It("should return the resource", func() {
			comp := handler.Resource()
			Expect(comp).ToNot(BeNil())
			Expect(comp).To(BeAssignableToTypeOf(&openmcpv1alpha1.Authentication{}))
		})

		It("should return empty label selectors for the namespace scoped admin role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.AdminNamespaceScopeRole)
			Expect(ls).To(BeNil())
		})

		It("should return empty label selectors for the namespace scoped view role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.ViewNamespaceScopeRole)
			Expect(ls).To(BeNil())
		})

		It("should return empty label selectors for the cluster scoped admin role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.AdminClusterScopeRole)
			Expect(ls).To(BeNil())
		})

		It("should return empty label selectors for the cluster scoped view role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.ViewClusterScopeRole)
			Expect(ls).To(BeNil())
		})
	})

	Context("Authorization", func() {
		var (
			converter components.ComponentConverter
			handler   *components.ComponentHandler
		)

		BeforeEach(func() {
			handler = components.Registry.GetComponent(openmcpv1alpha1.AuthorizationComponent)
			Expect(handler).ToNot(BeNil())
			converter = handler.Converter()
			Expect(converter).ToNot(BeNil())

		})

		It("should return the resource", func() {
			comp := handler.Resource()
			Expect(comp).ToNot(BeNil())
			Expect(comp).To(BeAssignableToTypeOf(&openmcpv1alpha1.Authorization{}))
		})

		It("should return empty label selectors for the namespace scoped admin role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.AdminNamespaceScopeRole)
			Expect(ls).To(BeNil())
		})

		It("should return empty label selectors for the namespace scoped view role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.ViewNamespaceScopeRole)
			Expect(ls).To(BeNil())
		})

		It("should return empty label selectors for the cluster scoped admin role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.AdminClusterScopeRole)
			Expect(ls).To(BeNil())
		})

		It("should return empty label selectors for the cluster scoped view role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.ViewClusterScopeRole)
			Expect(ls).To(BeNil())
		})
	})

	Context("APIServer", func() {
		var (
			converter components.ComponentConverter
			handler   *components.ComponentHandler
		)

		BeforeEach(func() {
			handler = components.Registry.GetComponent(openmcpv1alpha1.APIServerComponent)
			Expect(handler).ToNot(BeNil())
			converter = handler.Converter()
			Expect(converter).ToNot(BeNil())

		})

		It("should return the resource", func() {
			comp := handler.Resource()
			Expect(comp).ToNot(BeNil())
			Expect(comp).To(BeAssignableToTypeOf(&openmcpv1alpha1.APIServer{}))
		})

		It("should return empty label selectors for the namespace scoped admin role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.AdminNamespaceScopeRole)
			Expect(ls).To(BeNil())
		})

		It("should return empty label selectors for the namespace scoped view role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.ViewNamespaceScopeRole)
			Expect(ls).To(BeNil())
		})

		It("should return empty label selectors for the cluster scoped admin role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.AdminClusterScopeRole)
			Expect(ls).To(BeNil())
		})

		It("should return empty label selectors for the cluster scoped view role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.ViewClusterScopeRole)
			Expect(ls).To(BeNil())
		})
	})

	Context("CloudOrchestrator", func() {
		var (
			converter components.ComponentConverter
			handler   *components.ComponentHandler
		)

		BeforeEach(func() {
			handler = components.Registry.GetComponent(openmcpv1alpha1.CloudOrchestratorComponent)
			Expect(handler).ToNot(BeNil())
			converter = handler.Converter()
			Expect(converter).ToNot(BeNil())

		})

		It("should return the resource", func() {
			comp := handler.Resource()
			Expect(comp).ToNot(BeNil())
			Expect(comp).To(BeAssignableToTypeOf(&openmcpv1alpha1.CloudOrchestrator{}))
		})

		It("should return empty label selectors for the namespace scoped admin role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.AdminNamespaceScopeRole)
			Expect(ls).To(BeNil())
		})

		It("should return empty label selectors for the namespace scoped view role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.ViewNamespaceScopeRole)
			Expect(ls).To(BeNil())
		})

		It("should return two label selectors for the cluster scoped admin role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.AdminClusterScopeRole)
			Expect(ls).ToNot(BeNil())
			Expect(ls).To(HaveLen(2))
			Expect(ls).To(ConsistOf(
				metav1.LabelSelector{
					MatchLabels: map[string]string{
						components.CrossPlaneClusterScopedAdminMatchLabel: components.MatchLabelValue,
					},
				},
				metav1.LabelSelector{
					MatchLabels: map[string]string{
						components.CloudOrchestratorClusterScopedAdminMatchLabel: components.MatchLabelValue,
					},
				}))
		})

		It("should return two label selectors for the cluster scoped view role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.ViewClusterScopeRole)
			Expect(ls).ToNot(BeNil())
			Expect(ls).To(HaveLen(2))
			Expect(ls).To(ConsistOf(
				metav1.LabelSelector{
					MatchLabels: map[string]string{
						components.CrossPlaneClusterScopedViewMatchLabel: components.MatchLabelValue,
					},
				},
				metav1.LabelSelector{
					MatchLabels: map[string]string{
						components.CloudOrchestratorClusterScopedViewMatchLabel: components.MatchLabelValue,
					},
				}))
		})
	})

	Context("Landscaper", func() {
		var (
			converter components.ComponentConverter
			handler   *components.ComponentHandler
		)

		BeforeEach(func() {
			handler = components.Registry.GetComponent(openmcpv1alpha1.LandscaperComponent)
			Expect(handler).ToNot(BeNil())
			converter = handler.Converter()
			Expect(converter).ToNot(BeNil())

		})

		It("should return the resource", func() {
			comp := handler.Resource()
			Expect(comp).ToNot(BeNil())
			Expect(comp).To(BeAssignableToTypeOf(&openmcpv1alpha1.Landscaper{}))
		})

		It("should return one label selector for the namespace scoped admin role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.AdminNamespaceScopeRole)
			Expect(ls).ToNot(BeNil())
			Expect(ls).To(HaveLen(1))
			Expect(ls).To(ConsistOf(
				metav1.LabelSelector{
					MatchLabels: map[string]string{
						components.LandscaperNamespaceScopedAdminMatchLabel: components.MatchLabelValue,
					},
				}))
		})

		It("should return one label selector for the namespace scoped view role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.ViewNamespaceScopeRole)
			Expect(ls).ToNot(BeNil())
			Expect(ls).To(HaveLen(1))
			Expect(ls).To(ConsistOf(
				metav1.LabelSelector{
					MatchLabels: map[string]string{
						components.LandscaperNamespaceScopedViewMatchLabel: components.MatchLabelValue,
					},
				}))
		})

		It("should return empty label selectors for the cluster scoped admin role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.AdminClusterScopeRole)
			Expect(ls).To(BeNil())
		})

		It("should return empty label selectors for the cluster scoped view role", func() {
			ls := handler.LabelSelectorsForRole(openmcpv1alpha1.ViewClusterScopeRole)
			Expect(ls).To(BeNil())
		})
	})
})
