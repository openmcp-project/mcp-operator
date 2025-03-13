package gardener

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.tools.sap/CoLa/mcp-operator/internal/utils"
	componentutils "github.tools.sap/CoLa/mcp-operator/internal/utils/components"

	apiserverconfig "github.tools.sap/CoLa/mcp-operator/internal/controller/core/apiserver/config"
	apiserverhandler "github.tools.sap/CoLa/mcp-operator/internal/controller/core/apiserver/handler"

	"github.com/openmcp-project/controller-utils/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cconst "github.tools.sap/CoLa/mcp-operator/api/constants"
	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.tools.sap/CoLa/mcp-operator/api/errors"
	gardenv1beta1 "github.tools.sap/CoLa/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1"
	"github.tools.sap/CoLa/mcp-operator/api/external/gardener/pkg/apis/core/v1beta1/constants"
)

const (
	GardenerDeletionConfirmationAnnotation = "confirmation.gardener.cloud/deletion"
)

var _ apiserverhandler.APIServerHandler = &GardenerConnector{}

type GardenerConnector struct {
	apiserverconfig.CompletedMultiGardenerConfiguration
	Common        *apiserverconfig.CompletedCommonConfig
	APIServerType openmcpv1alpha1.APIServerType
}

func NewGardenerConnector(cc *apiserverconfig.CompletedCommonConfig, cfg *apiserverconfig.CompletedMultiGardenerConfiguration, apiServerType openmcpv1alpha1.APIServerType) (*GardenerConnector, openmcperrors.ReasonableError) {
	if cfg == nil {
		return nil, openmcperrors.WithReason(fmt.Errorf("APIServer handler for type 'Gardener' is not configured"), cconst.ReasonConfigurationProblem)
	}
	return &GardenerConnector{
		CompletedMultiGardenerConfiguration: *cfg,
		Common:                              cc,
		APIServerType:                       apiServerType,
	}, nil
}

// GetShoot tries to fetch the corresponding shoot cluster.
// If there is a shoot reference in the InternalControlPlane's status, but the shoot is not found, an error is returned unless inDeletion is true.
// If there is no shoot reference, the function searches for a shoot with a fitting back-reference and returns that, if found.
// Otherwise, nil is returned.
func (gc *GardenerConnector) GetShoot(ctx context.Context, as *openmcpv1alpha1.APIServer, inDeletion bool) (*gardenv1beta1.Shoot, openmcperrors.ReasonableError) {
	log := logging.FromContextOrPanic(ctx)
	var sh *gardenv1beta1.Shoot

	lc := ""
	if as.Spec.Internal != nil && as.Spec.Internal.GardenerConfig != nil {
		lc = as.Spec.Internal.GardenerConfig.LandscapeConfiguration
	}
	gls, gcfg, err := gc.LandscapeConfiguration(lc)
	if err != nil {
		return nil, openmcperrors.WithReason(err, cconst.ReasonConfigurationProblem)
	}

	uShoot, err := as.Status.GardenerStatus.GetShoot()
	if err != nil {
		return nil, openmcperrors.WithReason(err, cconst.ReasonShootIdentificationNotPossible)
	}
	if uShoot != nil {
		sh = &gardenv1beta1.Shoot{}
		sh.SetName(uShoot.GetName())
		sh.SetNamespace(uShoot.GetNamespace())
		log.Debug("Found shoot reference", "shoot", client.ObjectKeyFromObject(sh).String())
		if err := gls.Client.Get(ctx, client.ObjectKeyFromObject(sh), sh); err != nil {
			if inDeletion && apierrors.IsNotFound(err) {
				return nil, nil
			}
			return nil, openmcperrors.WithReason(err, cconst.ReasonGardenClusterInteractionProblem)
		}
	} else {
		// Check if the status has somehow been lost, but there is a Shoot referencing this ManagedControlPlane
		log.Debug("No reference to shoot found, searching for shoot with matching back-reference")
		shoots := &gardenv1beta1.ShootList{}
		if err := gls.Client.List(ctx, shoots, client.MatchingLabels{
			openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName:      as.Name,
			openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace: as.Namespace,
		}, client.InNamespace(gcfg.ProjectNamespace)); err != nil {
			return nil, openmcperrors.WithReason(err, cconst.ReasonGardenClusterInteractionProblem)
		}

		if len(shoots.Items) > 0 {
			if len(shoots.Items) > 1 {
				return nil, openmcperrors.WithReason(fmt.Errorf("found %d Shoots referencing ManagedControlPlane '%s', there should never be more than one", len(shoots.Items), client.ObjectKeyFromObject(as).String()), cconst.ReasonShootIdentificationNotPossible)
			}
			sh = &shoots.Items[0]
			log.Debug("Found a Shoot with a matching back-reference", "shoot", client.ObjectKeyFromObject(sh).String())
		}
	}

	return sh, nil
}

func (gc *GardenerConnector) HandleCreateOrUpdate(ctx context.Context, as *openmcpv1alpha1.APIServer, crateClient client.Client) (ctrl.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
	log := logging.FromContextOrPanic(ctx).WithName("GardenerConnector")
	ctx = logging.NewContext(ctx, log)

	lc := ""
	if as.Spec.Internal != nil && as.Spec.Internal.GardenerConfig != nil {
		lc = as.Spec.Internal.GardenerConfig.LandscapeConfiguration
	}
	gls, gcfg, err := gc.LandscapeConfiguration(lc)
	if err != nil {
		return ctrl.Result{}, nil, gardenerConditions(false, cconst.ReasonConfigurationProblem, err.Error()), openmcperrors.WithReason(err, cconst.ReasonConfigurationProblem)
	}

	// check if shoot already exists
	sh, errr := gc.GetShoot(ctx, as, false)
	if errr != nil {
		log.Error(errr, "error checking for corresponding shoot")
	}

	auditLogShootAnnotations, auditLogErr := gc.reconcileAuditLogResources(ctx, as, gc.GetShootName(sh, as, gcfg), crateClient, gls, gcfg)
	if auditLogErr != nil {
		return ctrl.Result{}, nil, gardenerConditions(false, cconst.ReasonAuditLogProblem, auditLogErr.Error()), auditLogErr
	}

	shootReady := false
	shootNotReadyMessage := ""
	var updateShootManifestInStatusFunc func(status *openmcpv1alpha1.APIServerStatus) error
	if sh == nil {
		log.Debug("No existing shoot found, creating a new one")
		sh = &gardenv1beta1.Shoot{}
		if err := gc.Shoot_v1beta1_from_APIServer_v1alpha1(ctx, as, sh); err != nil {
			return ctrl.Result{}, nil, gardenerConditions(false, cconst.ReasonConfigurationProblem, err.Error()), openmcperrors.WithReason(err, cconst.ReasonConfigurationProblem)
		}
		updateShootManifestInStatusFunc = func(status *openmcpv1alpha1.APIServerStatus) error {
			status.GardenerStatus = &openmcpv1alpha1.GardenerStatus{}
			return InjectShootManifestInGardenerStatus(status.GardenerStatus, sh)
		}
		if err := gls.Client.Create(ctx, sh); err != nil {
			return ctrl.Result{}, updateShootManifestInStatusFunc, gardenerConditions(false, cconst.ReasonGardenClusterInteractionProblem, err.Error()), openmcperrors.WithReason(err, cconst.ReasonGardenClusterInteractionProblem)
		}
	} else {
		log.Debug("Updating existing shoot", "shoot", client.ObjectKeyFromObject(sh).String())
		if err := gc.Shoot_v1beta1_from_APIServer_v1alpha1(ctx, as, sh); err != nil {
			return ctrl.Result{}, nil, gardenerConditions(false, cconst.ReasonConfigurationProblem, err.Error()), openmcperrors.WithReason(err, cconst.ReasonConfigurationProblem)
		}

		if sh.Annotations == nil {
			sh.Annotations = make(map[string]string)
		}

		for k, v := range auditLogShootAnnotations {
			sh.Annotations[k] = v
		}

		updateShootManifestInStatusFunc = func(status *openmcpv1alpha1.APIServerStatus) error {
			status.GardenerStatus = &openmcpv1alpha1.GardenerStatus{}
			return InjectShootManifestInGardenerStatus(status.GardenerStatus, sh)
		}
		if err := gls.Client.Update(ctx, sh); err != nil {
			if apierrors.IsConflict(err) {
				log.Error(err, "Conflict updating shoot")
				return ctrl.Result{Requeue: true}, updateShootManifestInStatusFunc, gardenerConditions(false, cconst.ReasonGardenClusterInteractionProblem, err.Error()), openmcperrors.WithReason(err, cconst.ReasonGardenClusterInteractionProblem)
			}

			log.Error(err, "Error updating shoot")
			return ctrl.Result{}, updateShootManifestInStatusFunc, gardenerConditions(false, cconst.ReasonGardenClusterInteractionProblem, err.Error()), openmcperrors.WithReason(err, cconst.ReasonGardenClusterInteractionProblem)
		}
		shootReady, shootNotReadyMessage = isShootReady(sh)
	}
	log = log.WithValues("shoot", client.ObjectKeyFromObject(sh).String())

	var adminAccess *openmcpv1alpha1.APIServerAccess
	res := ctrl.Result{}
	if shootReady {
		log.Debug("Shoot is ready")
		adminAccess, res.RequeueAfter, err = apiserverhandler.GetClusterAccess(ctx, gc.Common.ServiceAccountNamespace, gc.Common.AdminServiceAccountName, as.Status.AdminAccess, &gardenerClusterAccessEnabler{
			gardenClient: gls.Client,
			shoot:        sh,
		})
		if err != nil {
			err = fmt.Errorf("error creating kubeconfigs for shoot cluster: %w", err)
			return ctrl.Result{}, updateShootManifestInStatusFunc, gardenerConditions(false, cconst.ReasonAPIServerAccessProvisioningNotPossible, err.Error()), openmcperrors.WithReason(err, cconst.ReasonAPIServerAccessProvisioningNotPossible)
		}
	} else {
		log.Debug("Shoot is not ready yet, requeueing APIServer")
		res.RequeueAfter = 60 * time.Second
	}

	usf := func(status *openmcpv1alpha1.APIServerStatus) error {
		if updateShootManifestInStatusFunc != nil {
			if err := updateShootManifestInStatusFunc(status); err != nil {
				return err
			}
		}

		if status.ExternalAPIServerStatus == nil {
			status.ExternalAPIServerStatus = &openmcpv1alpha1.ExternalAPIServerStatus{}
		}
		for _, endpoint := range sh.Status.AdvertisedAddresses {
			switch endpoint.Name {
			case constants.AdvertisedAddressExternal:
				status.ExternalAPIServerStatus.Endpoint = endpoint.URL
			case constants.AdvertisedAddressInternal:
			case constants.AdvertisedAddressServiceAccountIssuer:
				status.ExternalAPIServerStatus.ServiceAccountIssuer = endpoint.URL
			default:
				log.Error(nil, "unexpected endpoint name in shoot's advertised addresses", "endpoint", endpoint.Name)
			}
		}

		if adminAccess != nil {
			status.AdminAccess = adminAccess
		}

		return nil
	}

	conRsn := ""
	conMsg := strings.Builder{}
	if sh.Status.LastOperation != nil {
		conMsg.WriteString(fmt.Sprintf("[%s: %s] %s", sh.Status.LastOperation.Type, sh.Status.LastOperation.State, sh.Status.LastOperation.Description))
	}
	if !shootReady {
		conRsn = cconst.ReasonWaitingForGardenerShoot
		if conMsg.Len() > 0 {
			conMsg.WriteString("\n")
		}
		conMsg.WriteString(shootNotReadyMessage)
	}

	return res, usf, gardenerConditions(shootReady, conRsn, conMsg.String()), nil
}

func (gc *GardenerConnector) HandleDelete(ctx context.Context, as *openmcpv1alpha1.APIServer, crateClient client.Client) (ctrl.Result, apiserverhandler.UpdateStatusFunc, []openmcpv1alpha1.ComponentCondition, openmcperrors.ReasonableError) {
	log := logging.FromContextOrPanic(ctx).WithName("GardenerConnector")
	ctx = logging.NewContext(ctx, log)
	// check if shoot already exists
	sh, err := gc.GetShoot(ctx, as, true)
	if err != nil {
		err = openmcperrors.Errorf("error fetching corresponding shoot: %w", err, err)
		return ctrl.Result{}, nil, gardenerConditions(false, cconst.ReasonGardenClusterInteractionProblem, err.Error()), err
	}

	if sh == nil {
		// shoot is gone, cleanup is finished
		log.Debug("Shoot has been deleted")
		return ctrl.Result{}, func(status *openmcpv1alpha1.APIServerStatus) error {
			status.AdminAccess = nil
			if status.GardenerStatus != nil {
				status.GardenerStatus.Shoot = nil
			}
			return nil
		}, gardenerConditions(true, "", "Shoot has been deleted."), nil
	}

	lc := ""
	if as.Spec.Internal != nil && as.Spec.Internal.GardenerConfig != nil {
		lc = as.Spec.Internal.GardenerConfig.LandscapeConfiguration
	}
	gls, gcfg, errr := gc.LandscapeConfiguration(lc)
	if errr != nil {
		return ctrl.Result{}, nil, gardenerConditions(false, cconst.ReasonConfigurationProblem, errr.Error()), openmcperrors.WithReason(errr, cconst.ReasonConfigurationProblem)
	}

	if as.Spec.GardenerConfig != nil {
		as.Spec.GardenerConfig.AuditLog = nil
	}
	// delete the Audit Log resources
	if _, err := gc.reconcileAuditLogResources(ctx, as, sh.Name, crateClient, gls, gcfg); err != nil {
		return ctrl.Result{}, nil, gardenerConditions(false, cconst.ReasonAuditLogProblem, err.Error()), err

	}

	if sh.DeletionTimestamp.IsZero() {
		// delete the shoot cluster
		log.Debug("Deleting shoot", "shoot", client.ObjectKeyFromObject(sh).String())
		if err := componentutils.PatchAnnotation(ctx, gls.Client, sh, GardenerDeletionConfirmationAnnotation, "true", componentutils.ANNOTATION_OVERWRITE); err != nil {
			errr := openmcperrors.WithReason(fmt.Errorf("error patching deletion confirmation annotation onto shoot: %w", err), cconst.ReasonGardenClusterInteractionProblem)
			return ctrl.Result{}, nil, gardenerConditions(false, errr.Reason(), errr.Error()), errr
		}
		if err := gls.Client.Delete(ctx, sh); err != nil {
			errr := openmcperrors.WithReason(fmt.Errorf("error deleting shoot: %w", err), cconst.ReasonGardenClusterInteractionProblem)
			return ctrl.Result{}, nil, gardenerConditions(false, errr.Reason(), errr.Error()), errr
		}
	}

	conMsg := "Waiting for shoot cluster to be deleted."
	if sh.Status.LastOperation != nil {
		conMsg = fmt.Sprintf("[%s: %s] %s", sh.Status.LastOperation.Type, sh.Status.LastOperation.State, sh.Status.LastOperation.Description)
	}

	return ctrl.Result{RequeueAfter: 60 * time.Second}, nil, gardenerConditions(false, cconst.ReasonWaitingForGardenerShoot, conMsg), nil
}

func isShootReady(sh *gardenv1beta1.Shoot) (bool, string) {
	if sh.Status.ObservedGeneration != sh.Generation {
		return false, "Shoot's observed generation does not match its generation, indicating that it has not yet been reconciled after the last changes have been applied."
	}
	if len(sh.Status.Conditions) == 0 {
		return false, "Shoot is missing conditions."
	}
	unhealthyConditions := []string{}
	for _, con := range sh.Status.Conditions {
		if con.Status != gardenv1beta1.ConditionTrue {
			unhealthyConditions = append(unhealthyConditions, string(con.Type))
		}
	}
	if len(unhealthyConditions) > 0 {
		return false, fmt.Sprintf("The following shoot conditions are not satisfied: %s", strings.Join(unhealthyConditions, ", "))
	}
	return true, ""
}

func isAuditLogEnabled(as *openmcpv1alpha1.APIServer) bool {
	return as.Spec.GardenerConfig != nil && as.Spec.GardenerConfig.AuditLog != nil
}

type AuditLogAnnotations map[string]string

// reconcileAuditLogResources reconciles the audit log resources for the given shoot in the Garden cluster.
// If the content of the audit log resources in the Crate cluster has changed, this function will return the annotations that are needed to be set on the shoot.
// Otherwise, the annotations will be nil.
func (gc *GardenerConnector) reconcileAuditLogResources(ctx context.Context, as *openmcpv1alpha1.APIServer, shootName string, crateClient client.Client, gls *apiserverconfig.CompletedGardenerLandscape, gcfg *apiserverconfig.CompletedGardenerConfiguration) (AuditLogAnnotations, openmcperrors.ReasonableError) {
	if isAuditLogEnabled(as) {
		resultPolicy, err := gc.createOrUpdateAuditLogPolicy(ctx, as, shootName, crateClient, gls, gcfg)
		if err != nil {
			return nil, openmcperrors.WithReason(err, cconst.ReasonGardenClusterInteractionProblem)
		}
		resultCreds, err := gc.createOrUpdateAuditLogCredentials(ctx, as, shootName, crateClient, gls, gcfg)
		if err != nil {
			return nil, openmcperrors.WithReason(err, cconst.ReasonGardenClusterInteractionProblem)
		}
		if resultPolicy != controllerutil.OperationResultNone || resultCreds != controllerutil.OperationResultNone {
			return AuditLogAnnotations{
				constants.GardenerOperation: constants.GardenerOperationReconcile,
			}, nil
		}
	} else {
		if err := gc.deleteAuditLogPolicy(ctx, shootName, gls, gcfg); err != nil {
			return nil, openmcperrors.WithReason(err, cconst.ReasonGardenClusterInteractionProblem)
		}
		if err := gc.deleteAuditLogCredentials(ctx, shootName, gls, gcfg); err != nil {
			return nil, openmcperrors.WithReason(err, cconst.ReasonGardenClusterInteractionProblem)
		}
	}
	return nil, nil
}

// createOrUpdateAuditLogPolicy creates or updates the audit log policy ConfigMap for the given shoot.
func (gc *GardenerConnector) createOrUpdateAuditLogPolicy(ctx context.Context, as *openmcpv1alpha1.APIServer, shootName string, crateClient client.Client, gls *apiserverconfig.CompletedGardenerLandscape, gcfg *apiserverconfig.CompletedGardenerConfiguration) (controllerutil.OperationResult, error) {
	cmCrate := &corev1.ConfigMap{}
	err := crateClient.Get(ctx, types.NamespacedName{Name: as.Spec.GardenerConfig.AuditLog.PolicyRef.Name, Namespace: as.Namespace}, cmCrate)
	if err != nil {
		return "", err
	}

	cmGarden := &corev1.ConfigMap{}
	cmGarden.SetName(utils.PrefixWithNamespace(shootName, "auditlog-policy"))
	cmGarden.SetNamespace(gcfg.ProjectNamespace)
	result, err := controllerutil.CreateOrUpdate(ctx, gls.Client, cmGarden, func() error {
		cmGarden.Data = cmCrate.Data
		return nil

	})
	return result, err

}

// createOrUpdateAuditLogCredentials creates or updates the audit log credentials Secret for the given shoot.
func (gc *GardenerConnector) createOrUpdateAuditLogCredentials(ctx context.Context, as *openmcpv1alpha1.APIServer, shootName string, crateClient client.Client, gls *apiserverconfig.CompletedGardenerLandscape, gcfg *apiserverconfig.CompletedGardenerConfiguration) (controllerutil.OperationResult, error) {
	secretCrate := &corev1.Secret{}
	err := crateClient.Get(ctx, types.NamespacedName{Name: as.Spec.GardenerConfig.AuditLog.SecretRef.Name, Namespace: as.Namespace}, secretCrate)
	if err != nil {
		return "", err
	}

	secretGarden := &corev1.Secret{}
	secretGarden.SetName(utils.PrefixWithNamespace(shootName, "auditlog-credentials"))
	secretGarden.SetNamespace(gcfg.ProjectNamespace)
	result, err := controllerutil.CreateOrUpdate(ctx, gls.Client, secretGarden, func() error {
		secretGarden.Data = secretCrate.Data
		secretGarden.Type = secretCrate.Type
		return nil

	})
	return result, err
}

// deleteAuditLogPolicy deletes the audit log policy ConfigMap for the given shoot.
func (gc *GardenerConnector) deleteAuditLogPolicy(ctx context.Context, shootName string, gls *apiserverconfig.CompletedGardenerLandscape, gcfg *apiserverconfig.CompletedGardenerConfiguration) error {
	cmGarden := &corev1.ConfigMap{}
	cmGarden.SetName(utils.PrefixWithNamespace(shootName, "auditlog-policy"))
	cmGarden.SetNamespace(gcfg.ProjectNamespace)
	err := gls.Client.Delete(ctx, cmGarden)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// deleteAuditLogCredentials deletes the audit log credentials Secret for the given shoot.
func (gc *GardenerConnector) deleteAuditLogCredentials(ctx context.Context, shootName string, gls *apiserverconfig.CompletedGardenerLandscape, gcfg *apiserverconfig.CompletedGardenerConfiguration) error {
	secretGarden := &corev1.Secret{}
	secretGarden.SetName(utils.PrefixWithNamespace(shootName, "auditlog-credentials"))
	secretGarden.SetNamespace(gcfg.ProjectNamespace)
	err := gls.Client.Delete(ctx, secretGarden)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

var _ apiserverhandler.ClusterAccessEnabler = &gardenerClusterAccessEnabler{}

type gardenerClusterAccessEnabler struct {
	gardenClient client.Client
	shoot        *gardenv1beta1.Shoot

	client        client.Client
	restCfg       *rest.Config
	isInitialized bool
}

func (g *gardenerClusterAccessEnabler) Init(ctx context.Context) error {
	if g.isInitialized {
		return nil
	}
	var err error
	g.client, g.restCfg, err = getTemporaryClientForShoot(ctx, g.gardenClient, g.shoot)
	return err
}

func (g *gardenerClusterAccessEnabler) Client() client.Client {
	return g.client
}

func (g *gardenerClusterAccessEnabler) RESTConfig() *rest.Config {
	return g.restCfg
}

func gardenerConditions(shootReady bool, reason, message string) []openmcpv1alpha1.ComponentCondition {
	conditions := []openmcpv1alpha1.ComponentCondition{
		componentutils.NewCondition(openmcpv1alpha1.APIServerComponent.HealthyCondition(), openmcpv1alpha1.ComponentConditionStatusFromBool(shootReady), reason, message),
	}
	return conditions
}

// InjectShootManifestInGardenerStatus takes a GardenerStatus pointer and a shoot object and injects the shoot manifest into the GardenerStatus.
// It removes some metadata fields as well as the shoot's status.
func InjectShootManifestInGardenerStatus(status *openmcpv1alpha1.GardenerStatus, shoot *gardenv1beta1.Shoot) error {
	uShoot := &unstructured.Unstructured{}
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(shoot)
	if err != nil {
		return fmt.Errorf("unable to convert shoot to unstructured object: %w", err)
	}
	uShoot.SetUnstructuredContent(data)
	// ensure type information is set
	uShoot.SetAPIVersion(gardenv1beta1.SchemeGroupVersion.String())
	uShoot.SetKind("Shoot")
	// delete fields that should not be part of the shoot manifest in the status
	uShoot.SetFinalizers(nil)
	uShoot.SetResourceVersion("")
	uShoot.SetCreationTimestamp(metav1.Time{})
	uShoot.SetGenerateName("")
	uShoot.SetGeneration(0)
	uShoot.SetManagedFields(nil)
	uShoot.SetDeletionGracePeriodSeconds(nil)
	uShoot.SetDeletionTimestamp(nil)
	uShoot.SetOwnerReferences(nil)
	// remove shoot status
	unstructured.RemoveNestedField(uShoot.Object, "status")

	status.Shoot = &runtime.RawExtension{Object: uShoot}
	return nil
}
