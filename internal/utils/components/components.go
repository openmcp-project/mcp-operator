package components

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	components "github.tools.sap/CoLa/mcp-operator/internal/components"
	"github.tools.sap/CoLa/mcp-operator/internal/utils"

	ctrl "sigs.k8s.io/controller-runtime"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openmcp-project/controller-utils/pkg/logging"

	cconst "github.tools.sap/CoLa/mcp-operator/api/constants"
	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.tools.sap/CoLa/mcp-operator/api/errors"
)

const (
	// ErrorReasonInvalidManagedControlPlaneLabels means that the resource does not have the expected labels in the expected formats.
	ErrorReasonInvalidManagedControlPlaneLabels = "InvalidManagedControlPlaneLabels"
)

// GetComponents iterates over the list of known components and tries to fetch each corresponding resource.
// The component-specific resources are expected to have the same name and namespace as the ManagedControlPlane.
// The resulting map contains only entries for component resources that were found. A missing resource will not result in an error.
func GetComponents[T components.ManagedComponent](reg components.ComponentRegistry[T], ctx context.Context, c client.Client, name, namespace string) (map[openmcpv1alpha1.ComponentType]T, error) {
	res := reg.GetKnownComponents()
	for ct, ch := range res {
		ch.Resource().SetName(name)
		ch.Resource().SetNamespace(namespace)
		if err := c.Get(ctx, client.ObjectKeyFromObject(ch.Resource()), ch.Resource()); err != nil {
			if apierrors.IsNotFound(err) {
				delete(res, ct)
				continue
			}
			return nil, fmt.Errorf("error getting resource '%s/%s' for component type '%s': %w", namespace, name, string(ct), err)
		}
	}
	return res, nil
}

// GetComponent fetches a single component resource from the cluster.
func GetComponent[T components.ManagedComponent](reg components.ComponentRegistry[T], ctx context.Context, c client.Client, component openmcpv1alpha1.ComponentType, name, namespace string) (T, error) {
	var zero T
	ch := reg.GetComponent(component)
	if reflect.DeepEqual(ch, zero) {
		return zero, fmt.Errorf("component '%s' is not in the list of known components", string(component))
	}
	ch.Resource().SetName(name)
	ch.Resource().SetNamespace(namespace)
	if err := c.Get(ctx, client.ObjectKeyFromObject(ch.Resource()), ch.Resource()); err != nil {
		if apierrors.IsNotFound(err) {
			return zero, nil
		}
		return zero, err
	}
	return ch, nil
}

var ErrNoControlPlaneGenerationLabel = openmcperrors.WithReason(fmt.Errorf("object does not have a '%s' label", openmcpv1alpha1.ManagedControlPlaneGenerationLabel), ErrorReasonInvalidManagedControlPlaneLabels)

type InvalidGenerationLabelValueError struct {
	value       string
	culpritIsIC bool
}

var _ openmcperrors.ReasonableError = &InvalidGenerationLabelValueError{}

func NewInvalidGenerationLabelValueError(value string, culpritIsIC bool) *InvalidGenerationLabelValueError {
	return &InvalidGenerationLabelValueError{
		value:       value,
		culpritIsIC: culpritIsIC,
	}
}
func (e *InvalidGenerationLabelValueError) Error() string {
	invalidLabel := openmcpv1alpha1.ManagedControlPlaneGenerationLabel
	if e.culpritIsIC {
		invalidLabel = openmcpv1alpha1.InternalConfigurationGenerationLabel
	}
	return fmt.Sprintf("value '%s' of label '%s' cannot be parsed into an int64", e.value, invalidLabel)
}
func (e *InvalidGenerationLabelValueError) Reason() string {
	return ErrorReasonInvalidManagedControlPlaneLabels
}

// GetCreatedFromGeneration reads the generation labels for the ManagedControlPlane and the InternalConfiguration from the object and returns their values as int64.
// If the object does not have an InternalConfiguration generation label, -1 is returned for its value.
// Returns an error if the ManagedControlPlane generation label does not exist or either label's value cannot be parsed into an int64.
func GetCreatedFromGeneration(obj client.Object) (int64, int64, openmcperrors.ReasonableError) {
	labels := obj.GetLabels()
	if labels == nil {
		return -1, -1, ErrNoControlPlaneGenerationLabel
	}
	rawCP, ok := labels[openmcpv1alpha1.ManagedControlPlaneGenerationLabel]
	if !ok {
		return -1, -1, ErrNoControlPlaneGenerationLabel
	}
	valCP, err := strconv.ParseInt(rawCP, 10, 64)
	if err != nil {
		return -1, -1, NewInvalidGenerationLabelValueError(rawCP, false)
	}
	var valIC int64 = -1
	rawIC, ok := labels[openmcpv1alpha1.InternalConfigurationGenerationLabel]
	if ok {
		valIC, err = strconv.ParseInt(rawIC, 10, 64)
		if err != nil {
			return -1, -1, NewInvalidGenerationLabelValueError(rawIC, true)
		}
	}
	return valCP, valIC, nil
}

// SetCreatedFromGeneration sets the managedcontrolplane generation label to the generation of the given ManagedControlPlane.
// If the passed-in InternalConfiguration is not nil, its generation is also added as a label.
func SetCreatedFromGeneration(obj client.Object, cp client.Object, ic client.Object) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	if !utils.IsNil(cp) {
		labels[openmcpv1alpha1.ManagedControlPlaneGenerationLabel] = fmt.Sprintf("%d", cp.GetGeneration())
	}
	if !utils.IsNil(ic) {
		labels[openmcpv1alpha1.InternalConfigurationGenerationLabel] = fmt.Sprintf("%d", ic.GetGeneration())
	} else {
		delete(labels, openmcpv1alpha1.InternalConfigurationGenerationLabel)
	}
	obj.SetLabels(labels)
}

// GenerateCreatedFromGenerationPatch returns a patch which sets the generation labels according to the given ManagedControlPlane.
// If cp is nil, the value of the corresponding generation value is defaulted to -1. This should never happen.
// if ic is nil, the generated patch will remove the corresponding generation label, if it exists.
func GenerateCreatedFromGenerationPatch(cp, ic client.Object, addReconcileAnnotation bool) client.Patch {
	cpLabelValue := `"-1"`
	if !utils.IsNil(cp) {
		cpLabelValue = fmt.Sprintf(`"%d"`, cp.GetGeneration())
	}
	icLabelValue := `null`
	if !utils.IsNil(ic) {
		icLabelValue = fmt.Sprintf(`"%d"`, ic.GetGeneration())
	}
	reconcileAnnotationPatch := ""
	if addReconcileAnnotation {
		reconcileAnnotationPatch = fmt.Sprintf(`,"annotations":{"%s":"%s"}`, openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueReconcile)
	}
	return client.RawPatch(types.MergePatchType, []byte(fmt.Sprintf(`{"metadata":{"labels":{"%s":%s,"%s":%s}%s}}`, openmcpv1alpha1.ManagedControlPlaneGenerationLabel, cpLabelValue, openmcpv1alpha1.InternalConfigurationGenerationLabel, icLabelValue, reconcileAnnotationPatch)))
}

// UpdateStatus updates the status of the given component resource.
// The passed-in reconcile result is returned unmodified to be able to call this function in a reconcile return statement.
// The ready, reconcileError, reason, and msg parameters are used to update the common status of the component resource.
// If the status of oldComponent and component differs, the changes are applied too.
// Note: In order to only update the common status, call this with comp.DeepCopy() as oldComponent and comp as component.
// oldComponent and component MUST NOT point to the same object!
func UpdateStatus[T components.Component](ctx context.Context, c client.Client, rr ReconcileResult[T]) (ctrl.Result, openmcperrors.ReasonableError) {
	if utils.IsNil(rr.OldComponent) {
		rr.OldComponent = rr.Component.DeepCopyObject().(T)
	}

	errs := openmcperrors.NewReasonableErrorList(rr.ReconcileError)

	cpGen, icGen, err := GetCreatedFromGeneration(rr.Component)
	if err != nil {
		errs.Append(err)
	}

	aggErr := errs.Aggregate()
	if aggErr != nil {
		if rr.Message != "" {
			rr.Message += "\n"
		}
		rr.Message += aggErr.Error()
		if aggErr.Reason() != "" {
			rr.Reason = aggErr.Reason()
		}
	}

	commonStatus := rr.Component.GetCommonStatus()
	cu := ConditionUpdater(commonStatus.Conditions, true).UpdateCondition(rr.Component.Type().ReconciliationCondition(), openmcpv1alpha1.ComponentConditionStatusFromBool(aggErr == nil), rr.Reason, rr.Message)
	for _, con := range rr.Conditions {
		cu.UpdateConditionFromTemplate(con)
	}
	reqCons := rr.Component.GetRequiredConditions()
	if reqCons != nil && reqCons.Has(rr.Component.Type().HealthyCondition()) && !cu.HasCondition(rr.Component.Type().HealthyCondition()) && rr.ReconcileError != nil {
		// if an error occured during the reconciliation and the component is expected to expose a <component>Healthy condition, which is missing, put a reconciliation error into the condition
		cu.UpdateCondition(rr.Component.Type().HealthyCondition(), openmcpv1alpha1.ComponentConditionStatusFalse, cconst.ReasonReconciliationError, cconst.MessageReconciliationError)
	}
	for eCon := range rr.Component.GetRequiredConditions() {
		if !cu.HasCondition(eCon) {
			cu.UpdateCondition(eCon, openmcpv1alpha1.ComponentConditionStatusUnknown, cconst.ReasonMissingExpectedCondition, "The component did not expose this condition.")
		}
	}
	commonStatus.Conditions = cu.Conditions()
	commonStatus.ObservedGenerations = openmcpv1alpha1.ObservedGenerations{
		Resource:              rr.Component.GetGeneration(),
		ManagedControlPlane:   cpGen,
		InternalConfiguration: icGen,
	}
	rr.Component.SetCommonStatus(commonStatus)

	oldStatus := getStatus(rr.OldComponent)
	status := getStatus(rr.Component)
	if !reflect.DeepEqual(oldStatus, status) {
		err := c.Status().Patch(ctx, rr.Component, client.MergeFrom(rr.OldComponent))
		if client.IgnoreNotFound(err) != nil {
			errs2 := openmcperrors.NewReasonableErrorList(fmt.Errorf("error patching status: %w", err), aggErr)
			return rr.Result, errs2.Aggregate()
		}
	}
	return rr.Result, aggErr
}

// The result of a reconciliation.
type ReconcileResult[T components.Component] struct {
	// The old component, before it was modified.
	// Basically, if OldComponent.Status differs from Component.Status, the status will be patched during UpdateStatus.
	// May be nil, in this case only the common status is updated.
	// Changes to anything except the status are ignored.
	OldComponent T
	// The current components.
	// If nil, UpdateStatus becomes a no-op.
	Component T
	// The result of the reconciliation.
	// Is simply passed through.
	Result ctrl.Result
	// The error that occurred during reconciliation, if any.
	ReconcileError openmcperrors.ReasonableError
	// The reason for the component's condition.
	// If empty, it is taken from the error, if any.
	Reason string
	// The message for the component's condition.
	// Potential error messages from the reconciliation error are appended.
	Message string
	// Conditions contains a list of conditions that should be updated on the component.
	// Note that this must not contain the <component>Reconciliation condition, as that one is constructed from this struct's other fields.
	// Also note that names of conditions are globally unique, so take care to avoid conflicts with other components.
	// Futhermore, all conditions on the component resource that are not included in this list anymore will be removed.
	// The lastTransition timestamp of the condition will be overwritten with the current time while updating.
	Conditions []openmcpv1alpha1.ComponentCondition
}

// LogRequeue logs a message with the given logger at the given verbosity if the currently reconciled object is requeued for another reconciliation.
func (rr *ReconcileResult[T]) LogRequeue(log logging.Logger, verbosity logging.LogLevel) {
	if rr.Result.Requeue || rr.Result.RequeueAfter > 0 {
		log.Log(verbosity, "Object requeued for reconciliation", "requeueAfter", rr.Result.RequeueAfter.String(), "reconcileAt", time.Now().Add(rr.Result.RequeueAfter).Format(time.RFC3339))
	}
}
