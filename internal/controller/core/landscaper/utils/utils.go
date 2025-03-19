package utils

import (
	"context"
	"fmt"

	laasv1alpha1 "github.com/gardener/landscaper-service/pkg/apis/core/v1alpha1"
	"github.com/openmcp-project/controller-utils/pkg/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

// GetCorrespondingLandscaperDeployment fetches the LandscaperDeployment belonging to the given ManagedControlPlane.
// If there is a reference but the referenced LandscaperDeployment is not found, an IsNotFound error is returned.
// If there is no reference, the method tries to find a LandscaperDeployment with a label pointing to the given ManagedControlPlane, returning nil - but no error - if none is found.
func GetCorrespondingLandscaperDeployment(ctx context.Context, laasClient client.Client, ls *openmcpv1alpha1.Landscaper) (*laasv1alpha1.LandscaperDeployment, error) {
	log := logging.FromContextOrPanic(ctx)
	var ld *laasv1alpha1.LandscaperDeployment

	if ls.Status.LandscaperDeploymentInfo != nil {
		// Try to get the referenced LandscaperDeployment
		ld = &laasv1alpha1.LandscaperDeployment{}
		ld.SetName(ls.Status.LandscaperDeploymentInfo.Name)
		ld.SetNamespace(ls.Status.LandscaperDeploymentInfo.Namespace)
		log.Debug("Found reference to LandscaperDeployment", "resource", client.ObjectKeyFromObject(ld).String())
		if err := laasClient.Get(ctx, client.ObjectKeyFromObject(ld), ld); err != nil {
			return nil, err
		}
	} else {
		// Check if the status has somehow been lost, but there is a LandscaperDeployment referencing this ManagedControlPlane
		log.Debug("No reference to LandscaperDeployment found, searching for resource with matching back-reference")
		lds := &laasv1alpha1.LandscaperDeploymentList{}
		if err := laasClient.List(ctx, lds, client.MatchingLabels{
			openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName:      ls.Name,
			openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace: ls.Namespace,
		}); err != nil {
			return nil, err
		}

		if len(lds.Items) > 0 {
			if len(lds.Items) > 1 {
				return nil, fmt.Errorf("found %d LandscaperDeployments referencing ManagedControlPlane '%s', there should never be more than one", len(lds.Items), client.ObjectKeyFromObject(ls).String())
			}
			ld = &lds.Items[0]
			log.Info("Reference is missing, but found a LandscaperDeployment with a matching back-reference", "resource", client.ObjectKeyFromObject(ld).String())
		}
	}

	return ld, nil
}
