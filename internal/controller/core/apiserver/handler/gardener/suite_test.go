package gardener_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	apiserverconfig "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/config"
	"github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/handler/gardener"
	"github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/schemes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	colactrlutil "github.com/openmcp-project/controller-utils/pkg/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	openmcptesting "github.com/openmcp-project/controller-utils/pkg/testing"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	authenticationv1alpha1 "github.com/openmcp-project/mcp-operator/api/external/gardener/pkg/apis/authentication/v1alpha1"
	gardenv1beta1 "github.com/openmcp-project/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"
	testutils "github.com/openmcp-project/mcp-operator/test/utils"
)

const (
	gardenCluster  = "garden"
	gardenCluster2 = "garden2"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Gardener Handler Test Suite")
}

var (
	defaultConfigSingleBytes     []byte
	defaultConfigMultiBytes      []byte
	defaultConfigSingle          *apiserverconfig.APIServerProviderConfiguration
	defaultConfigMulti           *apiserverconfig.APIServerProviderConfiguration
	completedDefaultConfigSingle *apiserverconfig.CompletedAPIServerProviderConfiguration
	completedDefaultConfigMulti  *apiserverconfig.CompletedAPIServerProviderConfiguration
	testGardenObjs               []client.Object
	testGardenObjs2              []client.Object
	testCrateObjs                []client.Object
	env                          *openmcptesting.ComplexEnvironment

	gardenClusterInterceptorFuncs = interceptor.Funcs{Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
		switch obj.(type) {
		case *gardenv1beta1.Shoot:
			// throw an error in case of missing deletion confirmation annotation to mimic Gardener webhook behavior
			if !colactrlutil.HasAnnotationWithValue(obj, gardener.GardenerDeletionConfirmationAnnotation, "true") {
				return fmt.Errorf("missing deletion confirmation annotation")
			}
		}
		// use default logic
		return c.Delete(ctx, obj, opts...)
	},
		SubResourceCreate: func(ctx context.Context, c client.Client, subResourceName string, obj, subResource client.Object, opts ...client.SubResourceCreateOption) error {
			switch subResourceName {
			case "adminkubeconfig":
				adminKubeconfigRequest, ok := subResource.(*authenticationv1alpha1.AdminKubeconfigRequest)
				if !ok {
					return fmt.Errorf("unexpected object type %T", subResource)
				}
				adminKubeconfigRequest.Status.Kubeconfig = []byte("fake")
				adminKubeconfigRequest.Status.ExpirationTimestamp = metav1.Time{Time: time.Now().Add(time.Hour)}
				return nil
			}
			// use default logic
			return c.SubResource(subResourceName).Create(ctx, obj, subResource, opts...)
		},
	}
)

var _ = BeforeSuite(func() {
	var err error

	// load test objects for the Garden cluster
	testGardenObjs, err = openmcptesting.LoadObjects(path.Join("testdata", "garden_cluster"), testutils.Scheme)
	Expect(err).ToNot(HaveOccurred())

	// load test objects for the 2nd Garden cluster
	testGardenObjs2, err = openmcptesting.LoadObjects(path.Join("testdata", "garden_cluster_2"), testutils.Scheme)
	Expect(err).ToNot(HaveOccurred())

	// load test objects for the Crate cluster
	testCrateObjs, err = openmcptesting.LoadObjects(path.Join("testdata", "crate_cluster"), testutils.Scheme)
	Expect(err).ToNot(HaveOccurred())

	// load base config for single Gardener configuration
	defaultConfigSingleBytes, err = os.ReadFile(path.Join("testdata", "default_config_single.yaml"))
	Expect(err).NotTo(HaveOccurred())

	// load base config for multi Gardener configuration
	defaultConfigMultiBytes, err = os.ReadFile(path.Join("testdata", "default_config_multi.yaml"))
	Expect(err).NotTo(HaveOccurred())
})

var _ = BeforeEach(func() {
	var err error

	// generate default config for single Gardener configuration
	defaultConfigSingle, err = apiserverconfig.LoadConfigFromBytes(defaultConfigSingleBytes)
	Expect(err).NotTo(HaveOccurred())

	// generate default config for multi Gardener configuration
	defaultConfigMulti, err = apiserverconfig.LoadConfigFromBytes(defaultConfigMultiBytes)
	Expect(err).NotTo(HaveOccurred())

	env = openmcptesting.NewComplexEnvironmentBuilder().
		WithFakeClient(testutils.CrateCluster, testutils.Scheme).
		WithInitObjects(testutils.CrateCluster, testCrateObjs...).
		WithFakeClient(gardenCluster, schemes.GardenerScheme).
		WithInitObjects(gardenCluster, testGardenObjs...).
		WithDynamicObjectsWithStatus(gardenCluster, testGardenObjs...).
		WithFakeClientBuilderCall(gardenCluster, "WithInterceptorFuncs", gardenClusterInterceptorFuncs).
		WithFakeClient(gardenCluster2, schemes.GardenerScheme).
		WithInitObjects(gardenCluster2, testGardenObjs2...).
		WithDynamicObjectsWithStatus(gardenCluster2, testGardenObjs2...).
		WithFakeClientBuilderCall(gardenCluster2, "WithInterceptorFuncs", gardenClusterInterceptorFuncs).
		Build()

	// complete the single config
	defaultConfigSingle.GardenerConfig.InjectGardenClusterClient("", env.Client(gardenCluster))
	completedDefaultConfigSingle, err = defaultConfigSingle.Complete(env.Ctx)
	Expect(err).NotTo(HaveOccurred())

	// complete the multi config
	defaultConfigMulti.GardenerConfig.InjectGardenClusterClient("default", env.Client(gardenCluster))
	defaultConfigMulti.GardenerConfig.InjectGardenClusterClient("extra", env.Client(gardenCluster2))
	completedDefaultConfigMulti, err = defaultConfigMulti.Complete(env.Ctx)
	Expect(err).NotTo(HaveOccurred())
})

func initGardenerHandlerTestSingle(apiServerType openmcpv1alpha1.APIServerType, configFlavor string, paths ...string) (*gardener.GardenerConnector, *openmcpv1alpha1.APIServer) {
	// load APIServer from file
	as := &openmcpv1alpha1.APIServer{}
	Expect(openmcptesting.LoadObject(as, paths...)).To(Succeed())

	// initialize handler
	con, err := gardener.NewGardenerConnector(completedDefaultConfigSingle.CompletedCommonConfig, completedDefaultConfigSingle.GardenerConfig, as.Spec.Type)
	Expect(err).NotTo(HaveOccurred())

	con.APIServerType = apiServerType
	as.Spec.Type = apiServerType
	as.Spec.Internal = &openmcpv1alpha1.APIServerInternalConfiguration{
		GardenerConfig: &openmcpv1alpha1.GardenerInternalConfiguration{
			LandscapeConfiguration: configFlavor,
		},
	}

	return con, as
}

func initGardenerHandlerTestMulti(apiServerType openmcpv1alpha1.APIServerType, configFlavor string, paths ...string) (*gardener.GardenerConnector, *openmcpv1alpha1.APIServer) {
	// load APIServer from file
	as := &openmcpv1alpha1.APIServer{}
	Expect(openmcptesting.LoadObject(as, paths...)).To(Succeed())

	// initialize handler
	con, err := gardener.NewGardenerConnector(completedDefaultConfigMulti.CompletedCommonConfig, completedDefaultConfigMulti.GardenerConfig, as.Spec.Type)
	Expect(err).NotTo(HaveOccurred())

	con.APIServerType = apiServerType
	as.Spec.Type = apiServerType
	as.Spec.Internal = &openmcpv1alpha1.APIServerInternalConfiguration{
		GardenerConfig: &openmcpv1alpha1.GardenerInternalConfiguration{
			LandscapeConfiguration: configFlavor,
		},
	}

	return con, as
}
