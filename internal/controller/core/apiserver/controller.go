package apiserver

import (
	"cmp"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openmcp-project/mcp-operator/internal/utils"
	componentutils "github.com/openmcp-project/mcp-operator/internal/utils/components"

	apiserverconfig "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/config"
	apiserverhandler "github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/handler"
	"github.com/openmcp-project/mcp-operator/internal/controller/core/apiserver/handler/gardener"

	"github.com/openmcp-project/controller-utils/pkg/logging"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
)

const ControllerName = "APIServer"

func (r *APIServerProvider) GetAPIServerHandlerForType(ctx context.Context, t openmcpv1alpha1.APIServerType, cfg apiserverconfig.CompletedAPIServerProviderConfiguration) (apiserverhandler.APIServerHandler, error) {
	log := logging.FromContextOrPanic(ctx)
	switch t {
	case openmcpv1alpha1.Gardener, openmcpv1alpha1.GardenerDedicated:
		log.Debug(fmt.Sprintf("APIServer has type %s, loading corresponding connector", string(t)))
		return gardener.NewGardenerConnector(cfg.CompletedCommonConfig, cfg.GardenerConfig, t)
	case "Fake":
		if r.FakeHandler != nil {
			return r.FakeHandler, nil
		}
	}
	return nil, fmt.Errorf("unknown API server type '%s'", string(t))
}

func NewAPIServerProvider(ctx context.Context, client client.Client, cfg *apiserverconfig.APIServerProviderConfiguration) (*APIServerProvider, error) {
	log, ctx := utils.InitializeControllerLogger(ctx, ControllerName)
	ccfg, err := cfg.Complete(ctx)
	if err != nil {
		return nil, fmt.Errorf("error completing config: %w", err)
	}

	if ccfg.GardenerConfig != nil {
		log.Info("APIServer handler for type 'Gardener' configured")
	}

	return &APIServerProvider{
		CompletedAPIServerProviderConfiguration: *ccfg,
		Client:                                  client,
	}, nil
}

// APIServerProvider reconciles a ManagedControlPlane object
type APIServerProvider struct {
	apiserverconfig.CompletedAPIServerProviderConfiguration

	// Client is the registration cluster client.
	Client client.Client

	// FakeHandler is a fake APIServerHandler for testing purposes.
	// It should only be non-nil in tests.
	FakeHandler apiserverhandler.APIServerHandler
}

// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=apiservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=apiservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=apiservers/finalizers,verbs=update

func (r *APIServerProvider) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log, ctx := utils.InitializeControllerLogger(ctx, ControllerName)
	log.Debug(cconst.MsgStartReconcile)

	rr := r.reconcile(ctx, req)
	rr.LogRequeue(log, logging.DEBUG)
	if rr.Component == nil {
		return rr.Result, rr.ReconcileError
	}
	return componentutils.UpdateStatus(ctx, r.Client, rr)
}

func (r *APIServerProvider) reconcile(ctx context.Context, req ctrl.Request) componentutils.ReconcileResult[*openmcpv1alpha1.APIServer] {
	log := logging.FromContextOrPanic(ctx)

	// get internal APIServer resource
	as := &openmcpv1alpha1.APIServer{}
	if err := r.Client.Get(ctx, req.NamespacedName, as); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Resource not found")
			return componentutils.ReconcileResult[*openmcpv1alpha1.APIServer]{}
		}
		return componentutils.ReconcileResult[*openmcpv1alpha1.APIServer]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("unable to get resource '%s' from cluster: %w", req.String(), err), cconst.ReasonCrateClusterInteractionProblem)}
	}

	// handle operation annotation
	if as.GetAnnotations() != nil {
		op, ok := as.GetAnnotations()[openmcpv1alpha1.OperationAnnotation]
		if ok {
			switch op {
			case openmcpv1alpha1.OperationAnnotationValueIgnore:
				log.Info("Ignoring resource due to ignore operation annotation")
				return componentutils.ReconcileResult[*openmcpv1alpha1.APIServer]{}
			case openmcpv1alpha1.OperationAnnotationValueReconcile:
				log.Debug("Removing reconcile operation annotation from resource")
				if err := componentutils.PatchAnnotation(ctx, r.Client, as, openmcpv1alpha1.OperationAnnotation, "", componentutils.ANNOTATION_DELETE); err != nil {
					return componentutils.ReconcileResult[*openmcpv1alpha1.APIServer]{ReconcileError: openmcperrors.WithReason(fmt.Errorf("error removing operation annotation: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
				}
			}
		}
	}

	deleteAPIServer := false
	deletionWaitingForDependenciesMsg := ""
	if !as.DeletionTimestamp.IsZero() {
		log.Info("Deleting APIServer")
		if componentutils.HasAnyDependencyFinalizer(as) {
			depString := strings.Join(sets.List(componentutils.GetDependents(as)), ", ")
			log.Info("APIServer cannot be deleted, because it still contains dependency finalizers", "dependingComponents", depString)
			deletionWaitingForDependenciesMsg = fmt.Sprintf("Deletion is waiting for the following dependencies to be removed: [%s]", depString)
		} else {
			deleteAPIServer = true
		}
	} else {
		log.Info("Triggering creation/update of APIServer")

		old := as.DeepCopy()
		if controllerutil.AddFinalizer(as, openmcpv1alpha1.APIServerComponent.Finalizer()) {
			if err := r.Client.Patch(ctx, as, client.MergeFrom(old)); err != nil {
				return componentutils.ReconcileResult[*openmcpv1alpha1.APIServer]{Component: as, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error patching finalizer on APIServer: %w", err), cconst.ReasonCrateClusterInteractionProblem)}
			}
		}
	}

	apiServerHandler, err := r.GetAPIServerHandlerForType(ctx, as.Spec.Type, r.CompletedAPIServerProviderConfiguration)
	if err != nil {
		return componentutils.ReconcileResult[*openmcpv1alpha1.APIServer]{Component: as, ReconcileError: openmcperrors.WithReason(fmt.Errorf("error getting APIServer handler: %w", err), cconst.ReasonConfigurationProblem)}
	}
	ctx = logging.NewContext(ctx, log.WithValues("apiServerType", string(as.Spec.Type)))

	old := as.DeepCopy()
	var res ctrl.Result
	var usf apiserverhandler.UpdateStatusFunc
	var cons []openmcpv1alpha1.ComponentCondition
	var errr openmcperrors.ReasonableError
	if !deleteAPIServer {
		res, usf, cons, errr = apiServerHandler.HandleCreateOrUpdate(ctx, as, r.Client)
	} else {
		res, usf, cons, errr = apiServerHandler.HandleDelete(ctx, as, r.Client)
	}
	errs := openmcperrors.NewReasonableErrorList(errr)

	if usf != nil {
		errs.Append(usf(&as.Status))
	}

	if deletionWaitingForDependenciesMsg != "" {
		// we are waiting for one or more dependencies to be deleted
		return componentutils.ReconcileResult[*openmcpv1alpha1.APIServer]{Component: as, OldComponent: old, Result: ctrl.Result{RequeueAfter: minExceptZero(res.RequeueAfter, 60*time.Second)}, ReconcileError: errs.Aggregate(), Reason: cconst.ReasonDeletionWaitingForDependingComponents, Message: deletionWaitingForDependenciesMsg, Conditions: cons}
	}

	if deleteAPIServer && componentutils.AllConditionsTrue(cons...) {
		old := as.DeepCopy()
		changed := controllerutil.RemoveFinalizer(as, openmcpv1alpha1.APIServerComponent.Finalizer())
		if changed {
			if err := r.Client.Patch(ctx, as, client.MergeFrom(old)); err != nil {
				errs.Append(fmt.Errorf("error removing finalizer from APIServer: %w", err))
			}
		}
	}

	return componentutils.ReconcileResult[*openmcpv1alpha1.APIServer]{Component: as, OldComponent: old, Result: res, ReconcileError: errs.Aggregate(), Conditions: cons}
}

// SetupWithManager sets up the controller with the Manager.
func (r *APIServerProvider) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&openmcpv1alpha1.APIServer{}).
		WithEventFilter(componentutils.DefaultComponentControllerPredicates()).
		Complete(r)
}

// minExceptZero works like the builtin 'min' function, but will only return the zero value if all arguments are zero.
func minExceptZero[T cmp.Ordered](x T, y ...T) T {
	var zero T
	min := x
	for _, v := range y {
		if v != zero && (min == zero || v < min) {
			min = v
		}
	}
	return min
}
