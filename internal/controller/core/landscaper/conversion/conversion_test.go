package conversion_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.tools.sap/CoLa/mcp-operator/internal/controller/core/landscaper/conversion"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

var _ = Describe("LandscaperDeployment_v1alpha1_from_Landscaper_v1alpha1", func() {
	It("returns nil when Landscaper is nil", func() {
		ld := conversion.LandscaperDeployment_v1alpha1_from_Landscaper_v1alpha1(nil, "apiServerKubeconfig")
		Expect(ld).To(BeNil())
	})

	It("returns LandscaperDeployment with correct name and namespace", func() {
		ls := &openmcpv1alpha1.Landscaper{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		ld := conversion.LandscaperDeployment_v1alpha1_from_Landscaper_v1alpha1(ls, "apiServerKubeconfig")
		Expect(ld.GetName()).To(Equal("test-name"))
		Expect(ld.GetNamespace()).To(Equal("test-namespace"))
	})

	It("returns LandscaperDeployment with correct labels", func() {
		ls := &openmcpv1alpha1.Landscaper{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		ld := conversion.LandscaperDeployment_v1alpha1_from_Landscaper_v1alpha1(ls, "apiServerKubeconfig")
		Expect(ld.GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName, "test-name"))
		Expect(ld.GetLabels()).To(HaveKeyWithValue(openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace, "test-namespace"))
	})

	It("returns LandscaperDeployment with correct APIServer Kubeconfig", func() {
		ls := &openmcpv1alpha1.Landscaper{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-name",
				Namespace: "test-namespace",
			},
		}
		ld := conversion.LandscaperDeployment_v1alpha1_from_Landscaper_v1alpha1(ls, "apiServerKubeconfig")
		Expect(ld.Spec.DataPlane.Kubeconfig).To(Equal("apiServerKubeconfig"))
	})
})

var _ = Describe("LandscaperConfig_v1alpha1_from_lsConfig_v1alpha1", func() {
	It("returns empty LandscaperConfiguration when source is empty", func() {
		src := openmcpv1alpha1.LandscaperConfiguration{}
		result := conversion.LandscaperConfig_v1alpha1_from_lsConfig_v1alpha1(src)
		Expect(result.Deployers).To(BeEmpty())
	})

	It("returns LandscaperConfiguration with same deployers as source", func() {
		src := openmcpv1alpha1.LandscaperConfiguration{
			Deployers: []string{"deployer1", "deployer2"},
		}
		result := conversion.LandscaperConfig_v1alpha1_from_lsConfig_v1alpha1(src)
		Expect(result.Deployers).To(Equal(src.Deployers))
	})
})
