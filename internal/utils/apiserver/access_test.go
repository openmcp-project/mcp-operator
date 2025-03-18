package apiserver_test

import (
	"os"
	"path"

	"github.com/openmcp-project/mcp-operator/internal/utils/apiserver"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	restclient "k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp-project/controller-utils/pkg/testing"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	"github.com/openmcp-project/mcp-operator/test/utils"
)

var _ = Describe("APIServerAccess", func() {

	var (
		apiServerAccess *apiserver.APIServerAccessImpl
		as              *openmcpv1alpha1.APIServer
	)

	BeforeEach(func() {
		kubeConfig, err := os.ReadFile(path.Join("testdata", "kubeconfig.yaml"))
		Expect(err).ToNot(HaveOccurred())
		env := testing.NewEnvironmentBuilder().WithFakeClient(utils.Scheme).Build()

		apiServerAccess = &apiserver.APIServerAccessImpl{}
		as = &openmcpv1alpha1.APIServer{
			Status: openmcpv1alpha1.APIServerStatus{
				AdminAccess: &openmcpv1alpha1.APIServerAccess{
					Kubeconfig: string(kubeConfig),
				},
			},
		}

		apiServerAccess.NewClient = func(config *restclient.Config, options client.Options) (client.Client, error) {
			return env.Client(), nil
		}
	})

	Context("GetAdminAccessClient", func() {
		It("returns error when GetAdminAccessClient fails", func() {
			as.Status.AdminAccess.Kubeconfig = ""
			_, err := apiServerAccess.GetAdminAccessClient(as, client.Options{})
			Expect(err).To(HaveOccurred())
		})

		It("returns client when GetAdminAccessConfig succeeds", func() {
			_, err := apiServerAccess.GetAdminAccessClient(as, client.Options{})
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("GetAdminAccessConfig", func() {
		It("returns error when admin access kubeconfig is not available", func() {
			as.Status.AdminAccess.Kubeconfig = ""
			_, err := apiServerAccess.GetAdminAccessConfig(as)
			Expect(err).To(HaveOccurred())
		})

		It("returns config when admin access kubeconfig is available", func() {
			_, err := apiServerAccess.GetAdminAccessConfig(as)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("GetAdminAccessRaw", func() {
		It("returns error when admin access kubeconfig is not available", func() {
			as.Status.AdminAccess.Kubeconfig = ""
			_, err := apiServerAccess.GetAdminAccessRaw(as)
			Expect(err).To(HaveOccurred())
		})

		It("returns kubeconfig when admin access kubeconfig is available", func() {
			kubeconfig, err := apiServerAccess.GetAdminAccessRaw(as)
			Expect(err).ToNot(HaveOccurred())
			Expect(kubeconfig).ToNot(BeEmpty())
		})
	})
})
