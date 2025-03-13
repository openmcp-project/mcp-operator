package utils

import (
	"os"
	"path"

	"github.tools.sap/CoLa/mcp-operator/internal/utils/apiserver"

	laasinstall "github.com/gardener/landscaper-service/pkg/apis/core/install"
	"github.com/openmcp-project/controller-utils/pkg/testing"
	cocorev1beta1 "github.tools.sap/cloud-orchestration/control-plane-operator/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	gardenauthenticationv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/external/gardener/pkg/apis/authentication/v1alpha1"
	gardenv1beta1 "github.tools.sap/CoLa/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"
	openmcpinstall "github.tools.sap/CoLa/mcp-operator/api/install"
)

var (
	Scheme *runtime.Scheme
)

func init() {
	Scheme = runtime.NewScheme()
	openmcpinstall.Install(Scheme)
	laasinstall.Install(Scheme)
	utilruntime.Must(cocorev1beta1.AddToScheme(Scheme))
	utilruntime.Must(gardenv1beta1.AddToScheme(Scheme))
	utilruntime.Must(gardenauthenticationv1alpha1.AddToScheme(Scheme))
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
}

const (
	CrateCluster     = "crate"
	APIServerCluster = "apiserver"
	LaaSCoreCluster  = "laas"
	COCoreCluster    = "cloudOrchestrator"
)

type ReconcilerWithAPIServerAccess interface {
	SetAPIServerAccess(access apiserver.APIServerAccess)
}

func DefaultTestSetupBuilder(testDirPathSegments ...string) *testing.ComplexEnvironmentBuilder {
	builder := testing.NewComplexEnvironmentBuilder().
		WithFakeClient(CrateCluster, Scheme)

	if len(testDirPathSegments) > 0 && !(len(testDirPathSegments) == 1 && testDirPathSegments[0] == "") {
		builder.WithInitObjectPath(CrateCluster, testDirPathSegments...)
		apiServerDir := path.Join(path.Join(testDirPathSegments...), "apiserver")
		_, err := os.Stat(apiServerDir)
		if err == nil {
			builder.WithInitObjectPath(APIServerCluster, apiServerDir)
		}
	}

	return builder
}
