package schemes

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	gardenauthenticationv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/external/gardener/pkg/apis/authentication/v1alpha1"
	gardenv1beta1 "github.tools.sap/CoLa/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"
)

var (
	GardenerScheme *runtime.Scheme
)

func init() {
	GardenerScheme = runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(GardenerScheme))
	utilruntime.Must(gardenv1beta1.AddToScheme(GardenerScheme))
	utilruntime.Must(gardenauthenticationv1alpha1.AddToScheme(GardenerScheme))
}
