package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

var _ = Describe("ManagedControlPlane Webhook", func() {

	Context("When deleting a ManagedControlPlane", func() {
		It("Should deny if the annotation is not set", func() {
			var err error

			namespace := string(uuid.NewUUID())
			err = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
			Expect(err).ShouldNot(HaveOccurred())

			mcp := &ManagedControlPlane{ObjectMeta: metav1.ObjectMeta{Name: "mcp", Namespace: namespace}}
			err = k8sClient.Create(ctx, mcp)
			Expect(err).ShouldNot(HaveOccurred())

			err = k8sClient.Delete(ctx, mcp)
			Expect(apierrors.IsForbidden(err)).Should(BeTrue())
		})

		It("Should admit the deletion if the annoation was set", func() {
			var err error

			namespace := string(uuid.NewUUID())
			err = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
			Expect(err).ShouldNot(HaveOccurred())

			annotations := map[string]string{
				ManagedControlPlaneDeletionConfirmationAnnotation: "true",
			}
			mcp := &ManagedControlPlane{ObjectMeta: metav1.ObjectMeta{Name: "mcp", Namespace: namespace, Annotations: annotations}}
			err = k8sClient.Create(ctx, mcp)
			Expect(err).ShouldNot(HaveOccurred())

			err = k8sClient.Delete(ctx, mcp)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("When updating a ManagedControlPlane", func() {

		It("Should deny updates to spec.desiredRegion", func() {
			var err error

			namespace := string(uuid.NewUUID())
			err = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
			Expect(err).ShouldNot(HaveOccurred())

			mcp := &ManagedControlPlane{ObjectMeta: metav1.ObjectMeta{Name: "mcp", Namespace: namespace}}
			err = k8sClient.Create(ctx, mcp)
			Expect(err).ShouldNot(HaveOccurred())

			mcp.Spec.CommonConfig = &CommonConfig{
				DesiredRegion: &RegionSpecification{
					Name:      "europe",
					Direction: "east",
				},
			}

			err = k8sClient.Update(ctx, mcp)
			Expect(err).ShouldNot(HaveOccurred())

			mcp.Spec.DesiredRegion.Direction = "west"

			err = k8sClient.Update(ctx, mcp)
			Expect(err).To(HaveOccurred())
			// shouldn't be deleted
			mcp.Spec.CommonConfig = nil

			err = k8sClient.Update(ctx, mcp)
			Expect(err).To(HaveOccurred())
		})

		It("Should deny update to spec.components.apiServer", func() {
			var err error

			namespace := string(uuid.NewUUID())
			err = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
			Expect(err).ShouldNot(HaveOccurred())

			mcp := &ManagedControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mcp",
					Namespace: namespace,
				},
				Spec: ManagedControlPlaneSpec{
					Components: ManagedControlPlaneComponents{
						APIServer: &APIServerConfiguration{
							Type: Gardener,
							GardenerConfig: &GardenerConfiguration{
								Region: "eu-west-1",
							},
						},
					},
				},
			}
			err = k8sClient.Create(ctx, mcp)
			Expect(err).ShouldNot(HaveOccurred())

			mcp.Spec.Components.APIServer.Type = GardenerDedicated

			err = k8sClient.Update(ctx, mcp)
			Expect(err).To(HaveOccurred())

			// cover GardnerConfig as well
			mcp.Spec.Components.APIServer.GardenerConfig.Region = "eu-east-2"

			err = k8sClient.Update(ctx, mcp)
			Expect(err).To(HaveOccurred())

			// shouldn't be deleted
			mcp.Spec.Components.APIServer = nil

			err = k8sClient.Update(ctx, mcp)
			Expect(err).To(HaveOccurred())
		})

	})

})
