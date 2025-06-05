package cloudorchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openmcp-project/mcp-operator/internal/utils"
	"github.com/openmcp-project/mcp-operator/internal/utils/components"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/openmcp-project/controller-utils/pkg/api"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/yaml"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	corev1beta1 "github.com/openmcp-project/control-plane-operator/api/v1beta1"
	"github.com/openmcp-project/controller-utils/pkg/logging"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	condApi "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

const (
	defaultNamespace string = "openmcp-system" // TODO make this configurable
	ControllerName   string = "CloudOrchestrator"
)

var (
	errFetchingCloudOrchestrator openmcperrors.ReasonableError = openmcperrors.WithReason(errors.New("unable to fetch CloudOrchestrator"), cconst.ReasonCrateClusterInteractionProblem)
	errDeletingControlPlane      openmcperrors.ReasonableError = openmcperrors.WithReason(errors.New("unable to delete the ControlPlane"), cconst.ReasonCOCoreClusterInteractionProblem)
	errModifyingControlPlane     openmcperrors.ReasonableError = openmcperrors.WithReason(errors.New("unable to create or update Cloud Orchestrator ControlPlane resource"), cconst.ReasonCOCoreClusterInteractionProblem)
)

func NewCloudOrchestratorController(crateClient, coreClient client.Client, coreCluster cluster.Cluster) *CloudOrchestratorReconciler {
	return &CloudOrchestratorReconciler{
		CoreCluster: coreCluster,
		CoreClient:  coreClient,
		CrateClient: crateClient,
	}
}

// CloudOrchestratorReconciler reconciles a CloudOrchestrator object
type CloudOrchestratorReconciler struct {
	CoreCluster cluster.Cluster
	CoreClient  client.Client
	CrateClient client.Client
}

// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=cloudorchestrators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=cloudorchestrators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=cloudorchestrators/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=apiservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=*,resources=*,verbs=*
// +kubebuilder:rbac:urls=*,verbs=*

func (r *CloudOrchestratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log, ctx := utils.InitializeControllerLogger(ctx, ControllerName)
	log.Debug(cconst.MsgStartReconcile)

	rr, cp, reason, message := r.reconcile(ctx, req)
	rr.LogRequeue(log, logging.DEBUG)
	if rr.Component == nil {
		return rr.Result, rr.ReconcileError
	}
	if rr.ReconcileError != nil {
		reason = cconst.ReasonReconciliationError
		message = cconst.MessageReconciliationError
	}
	rr.Conditions = cloudOrchestratorConditions(reason == "" || reason == cconst.ReasonDeletionWaitingForDependingComponents, reason, message, cp, rr.Conditions...)
	return components.UpdateStatus(ctx, r.CrateClient, rr)
}

func (r *CloudOrchestratorReconciler) reconcile(ctx context.Context, req ctrl.Request) (components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator], *corev1beta1.ControlPlane, string, string) {
	log := logging.FromContextOrPanic(ctx)

	// get CloudOrchestrator resource
	co := &openmcpv1alpha1.CloudOrchestrator{}
	if err := r.CrateClient.Get(ctx, req.NamespacedName, co); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("CloudOrchestrator not found")
			return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{}, nil, "", ""
		}
		return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{ReconcileError: openmcperrors.Join(errFetchingCloudOrchestrator, err)}, nil, "", ""
	}

	// handle operation annotation
	if co.GetAnnotations() != nil {
		op, ok := co.GetAnnotations()[openmcpv1alpha1.OperationAnnotation]
		if ok {
			switch op {
			case openmcpv1alpha1.OperationAnnotationValueIgnore:
				log.Info("Ignoring resource due to ignore operation annotation")
				return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{}, nil, "", ""
			case openmcpv1alpha1.OperationAnnotationValueReconcile:
				log.Debug("Removing reconcile operation annotation from resource")
				if err := components.PatchAnnotation(ctx, r.CrateClient, co, openmcpv1alpha1.OperationAnnotation, "", components.ANNOTATION_DELETE); err != nil {
					return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing operation annotation: %w", err), cconst.ReasonCrateClusterInteractionProblem)}, nil, "", ""
				}
			}
		}
	}

	// Get ControlPlane as it could exist already and contain conditions that should be exposed on the CloudOrchestrator resource
	coreControlPlane := &corev1beta1.ControlPlane{}
	err := r.CoreClient.Get(ctx, client.ObjectKey{Name: utils.PrefixWithNamespace(co.Namespace, co.Name)}, coreControlPlane)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error fetching CloudOrchestrator ControlPlane resource: %w", err), cconst.ReasonCOCoreClusterInteractionProblem)}, nil, "", ""
		}
		// ControlPlane not found
		coreControlPlane = nil
	}

	// checking for APIServer component
	log.Debug("Checking for APIServer dependency")
	ownCPGeneration, ownICGeneration, _ := components.GetCreatedFromGeneration(co)
	as := &openmcpv1alpha1.APIServer{}
	if err := r.CrateClient.Get(ctx, req.NamespacedName, as); err != nil {
		if !apierrors.IsNotFound(err) {
			return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error fetching APIServer resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}, coreControlPlane, "", ""
		}
		// APIServer not found
		as = nil
	}

	if as == nil || !components.IsDependencyReady(as, ownCPGeneration, ownICGeneration) {
		log.Info("APIServer not found or it isn't ready")
		return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, Result: ctrl.Result{RequeueAfter: 60 * time.Second}}, coreControlPlane, cconst.ReasonWaitingForDependencies, "Waiting for APIServer dependency to be ready."
	}
	log.Debug("APIServer dependency is ready")
	auth := &openmcpv1alpha1.Authentication{}
	auth.SetName(co.Name)
	auth.SetNamespace(co.Namespace)
	authz := &openmcpv1alpha1.Authorization{}
	authz.SetName(co.Name)
	authz.SetNamespace(co.Namespace)

	if as.Spec.Type != openmcpv1alpha1.Gardener && as.Spec.Type != openmcpv1alpha1.GardenerDedicated {
		log.Info("APIServer is not of type Gardener/GardenerDedicated")
		return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.WithReason(fmt.Errorf("APIServer dependency is ready, but the APIServer type is not supported"), cconst.ReasonInvalidAPIServerType)}, coreControlPlane, "", ""
	}

	if as.Status.AdminAccess == nil || as.Status.AdminAccess.Kubeconfig == "" {
		return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.WithReason(fmt.Errorf("APIServer dependency is ready, but no kubeconfig could be found in its status"), cconst.ReasonDependencyStatusInvalid)}, coreControlPlane, "", ""
	}

	if !co.DeletionTimestamp.IsZero() {
		// handle deletion
		log.Info("Deleting CloudOrchestrator")
		if components.HasAnyDependencyFinalizer(co) {
			depString := strings.Join(sets.List(components.GetDependents(co)), ", ")
			log.Info("CloudOrchestrator cannot be deleted, because it still contains dependency finalizers", "dependingComponents", depString)
			return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, Result: ctrl.Result{RequeueAfter: 60 * time.Second}}, coreControlPlane, cconst.ReasonDeletionWaitingForDependingComponents, fmt.Sprintf("Deletion is waiting for the following dependencies to be removed: [%s]", depString)
		}

		_, err := r.deleteControlPlane(ctx, co)
		if err != nil {
			return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.Join(errDeletingControlPlane, err)}, coreControlPlane, "", ""
		}

		if coreControlPlane != nil {
			oldCO := co.DeepCopy()
			return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{OldComponent: oldCO, Component: co, Reason: cconst.ReasonComponentIsInDeletion, Result: ctrl.Result{RequeueAfter: 10 * time.Second}}, coreControlPlane, "", ""
		}

		// remove dependency finalizer from APIServer resource
		if err = components.EnsureDependencyFinalizer(ctx, r.CrateClient, as, co, false); err != nil {
			return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing dependency finalizer from APIServer component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}, coreControlPlane, "", ""
		}
		// remove dependency finalizer from Authentication resource
		if err := components.EnsureDependencyFinalizer(ctx, r.CrateClient, auth, co, false); client.IgnoreNotFound(err) != nil {
			return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing dependency finalizer from Authentication component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}, coreControlPlane, "", ""
		}
		// remove dependency finalizer from Authorization resource
		if err := components.EnsureDependencyFinalizer(ctx, r.CrateClient, authz, co, false); client.IgnoreNotFound(err) != nil {
			return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing dependency finalizer from Authorization component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}, coreControlPlane, "", ""
		}

		// remove finalizer from CloudOrchestrator resource
		old := co.DeepCopy()
		changed := controllerutil.RemoveFinalizer(co, openmcpv1alpha1.CloudOrchestratorComponent.Finalizer())
		if changed {
			if err := r.CrateClient.Patch(ctx, co, client.MergeFrom(old)); err != nil {
				return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing finalizer from CloudOrchestrator: %w", err), cconst.ReasonCrateClusterInteractionProblem)}, coreControlPlane, "", ""
			}
		}
		return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{}, nil, "", ""
	}

	// handle creation/update
	log.Info("Triggering creation/update of CloudOrchestrator")

	old := co.DeepCopy()
	if controllerutil.AddFinalizer(co, openmcpv1alpha1.CloudOrchestratorComponent.Finalizer()) {
		log.Debug("Adding finalizer to CloudOrchestrator resource")
		if err := r.CrateClient.Patch(ctx, co, client.MergeFrom(old)); err != nil {
			return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error patching finalizer on CloudOrchestrator: %w", err), cconst.ReasonCrateClusterInteractionProblem)}, coreControlPlane, "", ""
		}
	}

	log.Debug("Ensuring dependency finalizer on APIServer resource")
	if err := components.EnsureDependencyFinalizer(ctx, r.CrateClient, as, co, true); err != nil {
		return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error setting dependency finalizer on APIServer component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}, coreControlPlane, "", ""
	}
	log.Debug("Ensuring dependency finalizer on Authentication resource")
	if err := components.EnsureDependencyFinalizer(ctx, r.CrateClient, auth, co, true); err != nil {
		return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error setting dependency finalizer on Authentication component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}, coreControlPlane, "", ""
	}
	log.Debug("Ensuring dependency finalizer on Authorization resource")
	if err := components.EnsureDependencyFinalizer(ctx, r.CrateClient, authz, co, true); err != nil {
		return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error setting dependency finalizer on Authorization component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}, coreControlPlane, "", ""
	}

	// ControlPlane from Core Cluster
	coreControlPlane = &corev1beta1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.PrefixWithNamespace(co.Namespace, co.Name),
		},
	}

	// create or update the CO ControlPlane with the configuration from the openmcpv1alpha1.CloudOrchestrator CR
	_, err = controllerutil.CreateOrUpdate(ctx, r.CoreClient, coreControlPlane, func() error {
		spec, err := convertToControlPlaneSpec(&co.Spec, &as.Status)
		if err != nil {
			return err
		}
		coreControlPlane.Spec = *spec

		// update labels
		labels, err := r.copyLabels(ctx, co)
		if err != nil {
			return err
		}
		coreControlPlane.Labels = labels
		return nil
	})
	errs := openmcperrors.NewReasonableErrorList()
	if err != nil {
		errs = errs.Append(openmcperrors.Join(errModifyingControlPlane, err))
		return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{Component: co, ReconcileError: errs.Aggregate()}, coreControlPlane, "", ""
	}

	// find out if the CO ControlPlane resource is Ready
	isReady := r.isCloudOrchestratorReady(coreControlPlane.Status)

	res := ctrl.Result{}
	reason := ""
	message := ""
	if !isReady {
		log.Debug("ControlPlane resource is not ready yet")
		reason = cconst.ReasonWaitingForCloudOrchestrator
		message = "The ControlPlane resource is not ready yet."
	}

	// update CO status
	old = co.DeepCopy()
	updateCloudOrchestratorStatus(co, coreControlPlane)
	return components.ReconcileResult[*openmcpv1alpha1.CloudOrchestrator]{OldComponent: old, Component: co, Result: res}, coreControlPlane, reason, message
}

func updateCloudOrchestratorStatus(co *openmcpv1alpha1.CloudOrchestrator, coreControlPlane *corev1beta1.ControlPlane) {
	co.Status.ComponentsEnabled = coreControlPlane.Status.ComponentsEnabled
	co.Status.ComponentsHealthy = coreControlPlane.Status.ComponentsHealthy
}

// SetupWithManager sets up the controller with the Manager.
func (r *CloudOrchestratorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openmcpv1alpha1.CloudOrchestrator{}).
		WatchesRawSource(source.Kind(r.CoreCluster.GetCache(), &corev1beta1.ControlPlane{}, handler.TypedEnqueueRequestsFromMapFunc(func(ctx context.Context, t *corev1beta1.ControlPlane) []reconcile.Request {
			mcpName := t.Labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName]
			mcpNamespace := t.Labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace]
			if mcpName == "" || mcpNamespace == "" {
				return nil
			}
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: mcpName, Namespace: mcpNamespace}},
			}
		}))).
		Complete(r)
}

// convertToControlPlaneSpec will return a v1beta1.ControlPlaneSpec from a openmcpv1alpha1.CloudOrchestratorSpec and
// a openmcpv1alpha1.APIServerStatus.
func convertToControlPlaneSpec(coSpec *openmcpv1alpha1.CloudOrchestratorSpec, apiServerStatus *openmcpv1alpha1.APIServerStatus) (*corev1beta1.ControlPlaneSpec, error) {
	m := map[string]any{
		"rbac": map[string]any{
			"roleRef": map[string]string{
				"name": openmcpv1alpha1.AdminClusterScopeRole,
			},
		},
	}
	fluxValues, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}

	jsonData, err := yaml.ToJSON([]byte(apiServerStatus.AdminAccess.Kubeconfig))
	if err != nil {
		return nil, err
	}
	controlPlaneSpec := &corev1beta1.ControlPlaneSpec{
		Target: corev1beta1.Target{
			Target: api.Target{
				Kubeconfig: &apiextensionsv1.JSON{Raw: jsonData},
			},
			FluxServiceAccount: corev1beta1.ServiceAccountReference{
				Name:      "co-flux-deployer",
				Namespace: defaultNamespace,
			},
		},
		ComponentsConfig: corev1beta1.ComponentsConfig{},
	}

	if coSpec.Crossplane != nil {
		controlPlaneSpec.Crossplane = &corev1beta1.CrossplaneConfig{
			Version:   coSpec.Crossplane.Version,
			Providers: convertCrossplaneProviders(coSpec.Crossplane.Providers),
		}
	}

	if coSpec.BTPServiceOperator != nil {
		controlPlaneSpec.BTPServiceOperator = &corev1beta1.BTPServiceOperatorConfig{
			Version: coSpec.BTPServiceOperator.Version,
		}
		controlPlaneSpec.CertManager = &corev1beta1.CertManagerConfig{
			Version: "1.16.1",
		}
	}

	if coSpec.ExternalSecretsOperator != nil {
		controlPlaneSpec.ExternalSecretsOperator = &corev1beta1.ExternalSecretsOperatorConfig{
			Version: coSpec.ExternalSecretsOperator.Version,
		}
	}

	if coSpec.Kyverno != nil {
		controlPlaneSpec.Kyverno = &corev1beta1.KyvernoConfig{
			Version: coSpec.Kyverno.Version,
		}
	}

	if coSpec.Flux != nil {
		controlPlaneSpec.Flux = &corev1beta1.FluxConfig{
			Version: coSpec.Flux.Version,
			Values:  &apiextensionsv1.JSON{Raw: fluxValues},
		}
	}

	return controlPlaneSpec, nil
}

// deleteControlPlane will delete the ManagedControlPlane at the CO Core cluster.
// The returned bool will be true if the ControlPlane still exists after the deletion attempt. Otherwise, it will be false.
func (r *CloudOrchestratorReconciler) deleteControlPlane(ctx context.Context, co *openmcpv1alpha1.CloudOrchestrator) (bool, error) {
	coreControlPlane := &corev1beta1.ControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.PrefixWithNamespace(co.Namespace, co.Name),
		},
	}

	err := r.CoreClient.Delete(ctx, coreControlPlane)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	return true, err
}

// isCloudOrchestratorReady will return true if the status of the CO ManagedControlPlane is Ready and the Components are healthy
func (r *CloudOrchestratorReconciler) isCloudOrchestratorReady(status corev1beta1.ControlPlaneStatus) bool {
	return condApi.IsStatusConditionTrue(status.Conditions, "Ready") && status.ComponentsHealthy == status.ComponentsEnabled
}

// convertCrossplaneProviders will convert a slice of openmcpv1alpha1.CrossplaneProviderConfig to a slice of corev1beta1.CrossplaneProviderConfig
func convertCrossplaneProviders(providers []*openmcpv1alpha1.CrossplaneProviderConfig) []*corev1beta1.CrossplaneProviderConfig {
	if providers == nil {
		return nil
	}

	converted := make([]*corev1beta1.CrossplaneProviderConfig, len(providers))
	for i, p := range providers {
		converted[i] = &corev1beta1.CrossplaneProviderConfig{
			Name:    p.Name,
			Version: p.Version,
		}
	}
	return converted

}

// copyLabels will return a map of labels that should be added to the CO ManagedControlPlane
func (r *CloudOrchestratorReconciler) copyLabels(ctx context.Context, co *openmcpv1alpha1.CloudOrchestrator) (map[string]string, error) {
	// copy project and workspace name over
	ns := &v1.Namespace{}
	if err := r.CrateClient.Get(ctx, client.ObjectKey{Name: co.Namespace}, ns); err != nil {
		return nil, err
	}

	labels := map[string]string{}

	copyMapEntries(labels, co.Labels, openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName)
	copyMapEntries(labels, co.Labels, openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace)
	copyMapEntries(labels, ns.Labels, "openmcp.cloud/project", "openmcp.cloud/workspace")

	return labels, nil
}
