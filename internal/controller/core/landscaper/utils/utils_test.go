package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openmcp-project/mcp-operator/internal/controller/core/landscaper/utils"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	testutils "github.com/openmcp-project/mcp-operator/test/utils"
)

var _ = Describe("GetCorrespondingLandscaperDeployment", func() {
	It("returns an error when the LandscaperDeployment is not found", func() {
		env := testutils.DefaultTestSetupBuilder().Build()
		ls := &openmcpv1alpha1.Landscaper{
			Status: openmcpv1alpha1.LandscaperStatus{
				LandscaperDeploymentInfo: &openmcpv1alpha1.LandscaperDeploymentInfo{
					Name:      "test",
					Namespace: "test",
				},
			},
		}
		ld, err := utils.GetCorrespondingLandscaperDeployment(env.Ctx, env.Client(testutils.CrateCluster), ls)
		Expect(ld).To(BeNil())
		Expect(err).ToNot(BeNil())
	})

	It("returns LandscaperDeployment when LandscaperDeploymentInfo is set", func() {
		env := testutils.DefaultTestSetupBuilder("testdata", "test-01").Build()
		ls := &openmcpv1alpha1.Landscaper{
			Status: openmcpv1alpha1.LandscaperStatus{
				LandscaperDeploymentInfo: &openmcpv1alpha1.LandscaperDeploymentInfo{
					Name:      "test",
					Namespace: "test",
				},
			},
		}
		ld, err := utils.GetCorrespondingLandscaperDeployment(env.Ctx, env.Client(testutils.CrateCluster), ls)
		Expect(err).To(BeNil())
		Expect(ld).ToNot(BeNil())
		Expect(ld.GetName()).To(Equal("test"))
		Expect(ld.GetNamespace()).To(Equal("test"))

	})

	It("returns LandscaperDeployment when LandscaperDeploymentInfo is not set but matching back-reference found", func() {
		env := testutils.DefaultTestSetupBuilder("testdata", "test-01").Build()
		ls := &openmcpv1alpha1.Landscaper{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
		}
		ld, err := utils.GetCorrespondingLandscaperDeployment(env.Ctx, env.Client(testutils.CrateCluster), ls)
		Expect(err).To(BeNil())
		Expect(ld).ToNot(BeNil())
		Expect(ld.GetName()).To(Equal("test"))
		Expect(ld.GetNamespace()).To(Equal("test"))
	})

	It("returns error when multiple LandscaperDeployments referencing ManagedControlPlane", func() {
		env := testutils.DefaultTestSetupBuilder("testdata", "test-02").Build()

		ls := &openmcpv1alpha1.Landscaper{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
		}
		ld, err := utils.GetCorrespondingLandscaperDeployment(env.Ctx, env.Client(testutils.CrateCluster), ls)
		Expect(ld).To(BeNil())
		Expect(err).ToNot(BeNil())
	})
})
