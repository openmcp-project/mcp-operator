package conversion

import (
	laasv1alpha1 "github.com/gardener/landscaper-service/pkg/apis/core/v1alpha1"

	"github.tools.sap/CoLa/mcp-operator/internal/utils"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

// LandscaperDeployment_v1alpha1_from_Landscaper_v1alpha1 generates a LandscaperDeployment based on the given ManagedControlPlane resource.
func LandscaperDeployment_v1alpha1_from_Landscaper_v1alpha1(ls *openmcpv1alpha1.Landscaper, apiServerKubeconfig string) *laasv1alpha1.LandscaperDeployment {
	if ls == nil {
		return nil
	}
	ld := &laasv1alpha1.LandscaperDeployment{}
	ld.SetName(ls.Name)
	ld.SetNamespace(ls.Namespace)

	// set backreference label
	labels := map[string]string{}
	labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName] = ls.Name
	labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace] = ls.Namespace
	ld.SetLabels(labels)

	// set spec
	ld.Spec.TenantId = utils.K8sNameHash(ls.Namespace)[:8]
	ld.Spec.Purpose = "undefined" // TODO
	ld.Spec.LandscaperConfiguration = LandscaperConfig_v1alpha1_from_lsConfig_v1alpha1(ls.Spec.LandscaperConfiguration)

	ld.Spec.DataPlane = &laasv1alpha1.DataPlane{
		Kubeconfig: apiServerKubeconfig,
	}

	return ld
}

func LandscaperConfig_v1alpha1_from_lsConfig_v1alpha1(src openmcpv1alpha1.LandscaperConfiguration) laasv1alpha1.LandscaperConfiguration {
	var deployers []string
	if src.Deployers != nil {
		deployers = make([]string, len(src.Deployers))
		for i := range deployers {
			deployers[i] = src.Deployers[i]
		}
	}

	return laasv1alpha1.LandscaperConfiguration{
		Deployers: deployers,
	}
}
