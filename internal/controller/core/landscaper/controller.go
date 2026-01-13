package landscaper

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/openmcp-project/mcp-operator/internal/utils"
	"github.com/openmcp-project/mcp-operator/internal/utils/apiserver"
	"github.com/openmcp-project/mcp-operator/internal/utils/components"

	mcpocfg "github.com/openmcp-project/mcp-operator/internal/config"
	"github.com/openmcp-project/mcp-operator/internal/controller/core/landscaper/conversion"
	lsutils "github.com/openmcp-project/mcp-operator/internal/controller/core/landscaper/utils"

	"github.com/openmcp-project/controller-utils/pkg/collections/maps"
	"github.com/openmcp-project/controller-utils/pkg/logging"
	laasv1alpha1 "github.com/openmcp-project/landscaper-service/pkg/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
)

const (
	ControllerName = "Landscaper"

	LandscaperReadyPhase = "Succeeded"
	LandscaperErrorPhase = "Failed"
)

func NewLandscaperConnector(crateClient, laasClient client.Client) *LandscaperConnector {
	return &LandscaperConnector{
		CrateClient:     crateClient,
		LaaSClient:      laasClient,
		ApiServerAccess: &apiserver.APIServerAccessImpl{},
	}
}

// LandscaperConnector reconciles a ManagedControlPlane object
type LandscaperConnector struct {
	CrateClient, LaaSClient client.Client
	ApiServerAccess         apiserver.APIServerAccess
}

// SetAPIServerAccess sets the ApiServerAccess implementation.
// Used for testing.
func (r *LandscaperConnector) SetAPIServerAccess(apiServerAccess apiserver.APIServerAccess) {
	r.ApiServerAccess = apiServerAccess
}

// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=managedcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=managedcontrolplanes/status,verbs=get;update;patch

func (r *LandscaperConnector) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log, ctx := utils.InitializeControllerLogger(ctx, ControllerName)
	log.Debug(cconst.MsgStartReconcile)

	rr := r.reconcile(ctx, req)
	rr.LogRequeue(log, logging.DEBUG)
	if rr.Component == nil {
		return rr.Result, rr.ReconcileError
	}
	if rr.ReconcileError != nil && len(rr.Conditions) == 0 { // shortcut so we don't have to add the same ready condition to each return statement (won't work anymore if we have multiple conditions)
		rr.Conditions = landscaperConditions(false, cconst.ReasonReconciliationError, cconst.MessageReconciliationError)
	}
	return components.UpdateStatus(ctx, r.CrateClient, rr)
}

func (r *LandscaperConnector) reconcile(ctx context.Context, req ctrl.Request) components.ReconcileResult[*openmcpv1alpha1.Landscaper] {
	log := logging.FromContextOrPanic(ctx)

	// get Landscaper resource
	ls := &openmcpv1alpha1.Landscaper{}
	if err := r.CrateClient.Get(ctx, req.NamespacedName, ls); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Resource not found")
			return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{}
		}
		return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("unable to get resource '%s' from cluster: %w", req.String(), err), cconst.ReasonCrateClusterInteractionProblem)}
	}

	// handle operation annotation
	if ls.GetAnnotations() != nil {
		op, ok := ls.GetAnnotations()[openmcpv1alpha1.OperationAnnotation]
		if ok {
			switch op {
			case openmcpv1alpha1.OperationAnnotationValueIgnore:
				log.Info("Ignoring resource due to ignore operation annotation")
				return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{}
			case openmcpv1alpha1.OperationAnnotationValueReconcile:
				log.Debug("Removing reconcile operation annotation from resource")
				if err := components.PatchAnnotation(ctx, r.CrateClient, ls, openmcpv1alpha1.OperationAnnotation, "", components.ANNOTATION_DELETE); err != nil {
					return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing operation annotation: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
				}
			}
		}
	}

	// checking for APIServer component
	log.Debug("Checking for APIServer dependency")
	ownCPGeneration, ownICGeneration, _ := components.GetCreatedFromGeneration(ls)
	as := &openmcpv1alpha1.APIServer{}
	as.SetName(ls.Name)
	as.SetNamespace(ls.Namespace)
	if err := r.CrateClient.Get(ctx, client.ObjectKeyFromObject(as), as); err != nil {
		if !apierrors.IsNotFound(err) {
			return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{Component: ls, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error fetching APIServer resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}
		// APIServer not found
		as = nil
	}
	if as == nil || !components.IsDependencyReady(as, ownCPGeneration, ownICGeneration) {
		log.Info("APIServer not found or it isn't ready")
		return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{Component: ls, Conditions: landscaperConditions(false, cconst.ReasonWaitingForDependencies, "Waiting for APIServer dependency to be ready."), Result: ctrl.Result{RequeueAfter: 60 * time.Second}}
	}
	log.Debug("APIServer dependency is ready")
	if as.Status.AdminAccess == nil || as.Status.AdminAccess.Kubeconfig == "" {
		return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{Component: ls, ReconcileError: openmcperrors.WithReason(fmt.Errorf("APIServer dependency is ready, but no kubeconfig could be found in its status"), cconst.ReasonDependencyStatusInvalid)}
	}
	auth := &openmcpv1alpha1.Authentication{}
	auth.SetName(ls.Name)
	auth.SetNamespace(ls.Namespace)
	authz := &openmcpv1alpha1.Authorization{}
	authz.SetName(ls.Name)
	authz.SetNamespace(ls.Namespace)

	deleteLandscaper := false
	if !ls.DeletionTimestamp.IsZero() {
		log.Info("Deleting Landscaper")
		if components.HasAnyDependencyFinalizer(ls) {
			depString := strings.Join(sets.List(components.GetDependents(ls)), ", ")
			log.Info("Landscaper cannot be deleted, because it still contains dependency finalizers", "dependingComponents", depString)
			return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{Component: ls, Conditions: landscaperConditions(true, cconst.ReasonDeletionWaitingForDependingComponents, fmt.Sprintf("Deletion is waiting for the following dependencies to be removed: [%s]", depString)), Result: ctrl.Result{RequeueAfter: 60 * time.Second}}
		}
		deleteLandscaper = true
	} else {
		log.Info("Triggering creation/update of Landscaper")

		old := ls.DeepCopy()
		if controllerutil.AddFinalizer(ls, openmcpv1alpha1.LandscaperComponent.Finalizer()) {
			log.Debug("Adding finalizer to Landscaper resource")
			if err := r.CrateClient.Patch(ctx, ls, client.MergeFrom(old)); err != nil {
				return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("error patching finalizer on Landscaper: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
			}
		}

		log.Debug("Ensuring dependency finalizer on APIServer resource")
		if err := components.EnsureDependencyFinalizer(ctx, r.CrateClient, as, ls, true); err != nil {
			return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{Component: ls, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error setting dependency finalizer on APIServer component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}
		log.Debug("Ensuring dependency finalizer on Authentication resource")
		if err := components.EnsureDependencyFinalizer(ctx, r.CrateClient, auth, ls, true); err != nil {
			return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{Component: ls, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error setting dependency finalizer on Authentication component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}
		log.Debug("Ensuring dependency finalizer on Authorization resource")
		if err := components.EnsureDependencyFinalizer(ctx, r.CrateClient, authz, ls, true); err != nil {
			return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{Component: ls, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error setting dependency finalizer on Authorization component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}
	}

	ld, err := lsutils.GetCorrespondingLandscaperDeployment(ctx, r.LaaSClient, ls)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{Component: ls, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error trying to fetch corresponding LandscaperDeployment: %w", err), cconst.ReasonLaaSCoreClusterInteractionProblem)}
		}
		// This means that the control plane has a reference to a LandscaperDeployment which doesn't exist.
		// That shouldn't happen, but if we error out here, we prevent the system from recovering from a lost LandscaperDeployment.
		log.Info("Referenced LandscaperDeployment does not exist")
	}

	var res ctrl.Result
	var ready bool
	var reason string
	var v2cons []openmcpv1alpha1.ComponentCondition
	var errr openmcperrors.ReasonableError
	old := ls.DeepCopy()
	if mcpocfg.Config.Architecture.DecideVersion(ls) == openmcpv1alpha1.ArchitectureV2 {
		// v2 logic
		log.Info("Using v2 logic for APIServer")
		if deleteLandscaper {
			res, ready, v2cons, errr = r.v2HandleDelete(ctx, ls)
		} else {
			res, ready, v2cons, errr = r.v2HandleCreateOrUpdate(ctx, ls)
		}
		if !ready {
			reason = cconst.ReasonWaitingForLaaS
		}
	} else {
		// v1 logic
		if deleteLandscaper {
			res, ready, reason, errr = r.handleDelete(ctx, ls, ld)
		} else {
			res, ready, reason, errr = r.handleCreateOrUpdate(ctx, ls, ld, as)
		}
	}
	errs := openmcperrors.NewReasonableErrorList(errr)

	if deleteLandscaper && ready {
		// remove dependency finalizer from APIServer resource
		if err := components.EnsureDependencyFinalizer(ctx, r.CrateClient, as, ls, false); err != nil {
			return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{OldComponent: old, Component: ls, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing dependency finalizer from APIServer component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}
		// remove dependency finalizer from Authentication resource
		if err := components.EnsureDependencyFinalizer(ctx, r.CrateClient, auth, ls, false); client.IgnoreNotFound(err) != nil {
			return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{OldComponent: old, Component: ls, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing dependency finalizer from Authentication component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}
		// remove dependency finalizer from Authorization resource
		if err := components.EnsureDependencyFinalizer(ctx, r.CrateClient, authz, ls, false); client.IgnoreNotFound(err) != nil {
			return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{OldComponent: old, Component: ls, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing dependency finalizer from Authorization component resource: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
		}

		// remove finalizer from Landscaper resource
		old := ls.DeepCopy()
		changed := controllerutil.RemoveFinalizer(ls, openmcpv1alpha1.LandscaperComponent.Finalizer())
		if changed {
			if err := r.CrateClient.Patch(ctx, ls, client.MergeFrom(old)); err != nil {
				errs.Append(fmt.Errorf("error removing finalizer from Landscaper: %w", err))
			}
		}
	}

	cons := landscaperConditions(ready, reason, "")
	if ld != nil {
		cons[0].Message = fmt.Sprintf("LandscaperDeployment phase: %s", ld.Status.Phase)
		if !ready && errr == nil && ld.Status.LastError != nil {
			cons[0].Message = fmt.Sprintf("[%s] %s - %s", ld.Status.LastError.Operation, ld.Status.LastError.Reason, ld.Status.LastError.Message)
		}
	}
	cons = append(cons, v2cons...)
	return components.ReconcileResult[*openmcpv1alpha1.Landscaper]{OldComponent: old, Component: ls, Result: res, Reason: reason, ReconcileError: errs.Aggregate(), Conditions: cons}
}

func (r *LandscaperConnector) handleCreateOrUpdate(ctx context.Context, ls *openmcpv1alpha1.Landscaper, ld *laasv1alpha1.LandscaperDeployment, as *openmcpv1alpha1.APIServer) (ctrl.Result, bool, string, openmcperrors.ReasonableError) {
	log := logging.FromContextOrPanic(ctx)

	apiServerKubeconfig, err := r.ApiServerAccess.GetAdminAccessRaw(as)
	if err != nil {
		return ctrl.Result{}, false, "", openmcperrors.WithReason(err, cconst.ReasonLaaSCoreClusterInteractionProblem)
	}

	generatedLD := conversion.LandscaperDeployment_v1alpha1_from_Landscaper_v1alpha1(ls, apiServerKubeconfig)
	ldUpToDate := false

	if ld == nil {
		// no existing LandscaperDeployment has been found
		// let's create a new one
		ld = generatedLD
		log = log.WithValues("ldNamespace", ld.Namespace, "ldName", ld.Name)

		// check if namespace exists and create if necessary
		targetNamespace := &corev1.Namespace{}
		targetNamespace.SetName(ld.Namespace)
		targetNamespace.SetLabels(map[string]string{
			openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelName:      ls.Name,
			openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelNamespace: ls.Namespace,
		})
		if err := r.LaaSClient.Get(ctx, client.ObjectKeyFromObject(targetNamespace), targetNamespace); err != nil {
			if apierrors.IsNotFound(err) {
				log.Debug("Namespace for LandscaperDeployment does not exist, creating it")
				if err := r.LaaSClient.Create(ctx, targetNamespace); err != nil {
					return ctrl.Result{}, false, "", openmcperrors.WithReason(err, cconst.ReasonLaaSCoreClusterInteractionProblem)
				}
			} else {
				return ctrl.Result{}, false, "", openmcperrors.WithReason(err, cconst.ReasonLaaSCoreClusterInteractionProblem)
			}
		}

		log.Info("Creating LandscaperDeployment")
		if err := r.LaaSClient.Create(ctx, ld); err != nil {
			return ctrl.Result{}, false, "", openmcperrors.WithReason(err, cconst.ReasonLaaSCoreClusterInteractionProblem)
		}
	} else {
		// merge/overwrite values of existing LandscaperDeployment with the generated ones
		log = log.WithValues("ldNamespace", ld.Namespace, "ldName", ld.Name)
		changed := false
		wrongLabels := false
		for k, v := range generatedLD.GetLabels() {
			val, exists := ld.GetLabels()[k]
			if !exists || val != v {
				wrongLabels = true
				break
			}
		}
		if wrongLabels {
			ld.SetLabels(maps.Merge(ld.GetLabels(), generatedLD.GetLabels()))
			changed = true
		}
		if !reflect.DeepEqual(ld.Spec, generatedLD.Spec) {
			ld.Spec = generatedLD.Spec
			changed = true
		}
		if changed {
			log.Info("Updating existing LandscaperDeployment", "ldNamespace", ld.Namespace, "ldName", ld.Name)
			if err := r.LaaSClient.Update(ctx, ld); err != nil {
				return ctrl.Result{}, false, "", openmcperrors.WithReason(err, cconst.ReasonLaaSCoreClusterInteractionProblem)
			}
		} else {
			ldUpToDate = ld.Status.ObservedGeneration == ld.Generation && ld.Status.Phase == LandscaperReadyPhase
			if ldUpToDate {
				log.Info("LandscaperDeployment is up-to-date")
			} else {
				log.Info("Waiting for LandscaperDeployment to become ready")
			}
		}
	}

	ls.Status.LandscaperDeploymentInfo = &openmcpv1alpha1.LandscaperDeploymentInfo{
		Name:      ld.GetName(),
		Namespace: ld.GetNamespace(),
	}

	var requeueAfter time.Duration
	reason := ""
	if !ldUpToDate {
		requeueAfter = 30 * time.Second
		reason = cconst.ReasonWaitingForLaaS
	}
	return ctrl.Result{RequeueAfter: requeueAfter}, ldUpToDate, reason, nil
}

func (r *LandscaperConnector) handleDelete(ctx context.Context, ls *openmcpv1alpha1.Landscaper, ld *laasv1alpha1.LandscaperDeployment) (ctrl.Result, bool, string, openmcperrors.ReasonableError) {
	log := logging.FromContextOrPanic(ctx)
	if ld != nil {
		log = log.WithValues("ldNamespace", ld.Namespace, "ldName", ld.Name)
		log.Info("LandscaperDeployment still exists, deleting it")
		// remove the LandscaperDeployment
		if err := r.LaaSClient.Delete(ctx, ld); err != nil {
			return ctrl.Result{}, false, "", openmcperrors.WithReason(err, cconst.ReasonLaaSCoreClusterInteractionProblem)
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, false, cconst.ReasonWaitingForLaaS, nil
	}
	// LandscaperDeployment is gone
	log.Debug("Corresponding LandscaperDeployment is deleted")
	ls.Status.LandscaperDeploymentInfo = nil
	return ctrl.Result{}, true, "", nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LandscaperConnector) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openmcpv1alpha1.Landscaper{}, builder.WithPredicates(components.DefaultComponentControllerPredicates())).
		Watches(&openmcpv1alpha1.APIServer{}, &handler.EnqueueRequestForObject{}, builder.WithPredicates(components.StatusChangedPredicate{})).
		Complete(r)
}

func landscaperConditions(ready bool, reason, message string) []openmcpv1alpha1.ComponentCondition {
	return []openmcpv1alpha1.ComponentCondition{
		components.NewCondition(openmcpv1alpha1.LandscaperComponent.HealthyCondition(), openmcpv1alpha1.ComponentConditionStatusFromBool(ready), reason, message),
	}
}
