package managedcontrolplane

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/openmcp-project/mcp-operator/internal/components"
	"github.com/openmcp-project/mcp-operator/internal/utils"
	componentutils "github.com/openmcp-project/mcp-operator/internal/utils/components"

	"github.com/openmcp-project/controller-utils/pkg/collections/filters"
	"github.com/openmcp-project/controller-utils/pkg/collections/maps"
	openmcpctrlutil "github.com/openmcp-project/controller-utils/pkg/controller"
	"github.com/openmcp-project/controller-utils/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	cconst "github.com/openmcp-project/mcp-operator/api/constants"
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

const ControllerName = "ManagedControlPlane"

// isMCPKeyFilter is a filter that returns true for all map keys that are prefixed with the ManagedControlPlane base domain.
var isMCPKeyFilter = filters.ApplyToNthArgument(0, filters.Wrap(strings.HasPrefix, map[int]any{1: openmcpv1alpha1.BaseDomain}))

// ManagedControlPlaneController reconciles a ManagedControlPlane object
type ManagedControlPlaneController struct {
	Client client.Client
}

func NewManagedControlPlaneController(c client.Client) *ManagedControlPlaneController {
	return &ManagedControlPlaneController{
		Client: c,
	}
}

// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=managedcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=managedcontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.openmcp.cloud,resources=managedcontrolplanes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ManagedControlPlaneController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log, ctx := utils.InitializeControllerLogger(ctx, ControllerName)
	log.Debug(cconst.MsgStartReconcile)

	cp := &openmcpv1alpha1.ManagedControlPlane{}
	if err := r.Client.Get(ctx, req.NamespacedName, cp); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Resource not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// handle operation annotation
	hadReconcileAnnotation := false
	if cp.GetAnnotations() != nil {
		op, ok := cp.GetAnnotations()[openmcpv1alpha1.OperationAnnotation]
		if ok {
			switch op {
			case openmcpv1alpha1.OperationAnnotationValueIgnore:
				log.Info("Ignoring resource due to ignore operation annotation")
				return ctrl.Result{}, nil
			case openmcpv1alpha1.OperationAnnotationValueReconcile:
				hadReconcileAnnotation = true
				log.Debug("Removing reconcile operation annotation from resource")
				if err := componentutils.PatchAnnotation(ctx, r.Client, cp, openmcpv1alpha1.OperationAnnotation, "", componentutils.ANNOTATION_DELETE); err != nil {
					return ctrl.Result{}, fmt.Errorf("error removing operation annotation: %w", err)
				}
			}
		}
	}

	// fetch MCP namespace for project/workspace metadata
	ns := &corev1.Namespace{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: cp.Namespace}, ns); err != nil {
		// this is not crucial for the reconciliation, so we just log the error
		log.Error(err, "unable to fetch MCP namespace")
		ns = nil
	}

	// check if an InternalConfiguration resource exists for this ManagedControlPlane
	icfg := &openmcpv1alpha1.InternalConfiguration{}
	icfg.SetName(cp.Name)
	icfg.SetNamespace(cp.Namespace)
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(icfg), icfg); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("error fetching InternalConfiguration '%s/%s': %w", icfg.Namespace, icfg.Name, err)
		}
		icfg = nil
	} else {
		// ensure OwnerReference
		log.Debug("Corresponding InternalConfiguration found")
		oIdx, err := openmcpctrlutil.HasOwnerReference(icfg, cp, r.Client.Scheme())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("error checking for owner reference on InternalConfiguration object: %w", err)
		}
		if oIdx < 0 {
			// add OwnerReference
			log.Debug("Patching OwnerReference into InternalConfiguration")
			icfgOld := icfg.DeepCopy()
			if err := controllerutil.SetControllerReference(cp, icfg, r.Client.Scheme()); err != nil {
				return ctrl.Result{}, fmt.Errorf("error setting owner reference on InternalConfiguration object: %w", err)
			}
			if err := r.Client.Patch(ctx, icfg, client.MergeFrom(icfgOld)); err != nil {
				return ctrl.Result{}, fmt.Errorf("error patching owner reference on InternalConfiguration object: %w", err)
			}
		}
	}

	// handle deployment or deletion
	var cons []openmcpv1alpha1.ManagedControlPlaneComponentCondition
	var res ctrl.Result
	var err error
	inDeletion := !cp.DeletionTimestamp.IsZero()
	if !inDeletion {
		log.Info("Handling creation/update of ManagedControlPlane")
		cons, res, err = r.handleCreateOrUpdate(ctx, cp, icfg, ns, hadReconcileAnnotation)
	} else {
		log.Info("Handling deletion of ManagedControlPlane")
		cons, res, err = r.handleDelete(ctx, cp, ns, hadReconcileAnnotation)
	}

	// set ManagedControlPlane meta status
	cp.Status.ObservedGeneration = cp.Generation
	cp.Status.Status = openmcpv1alpha1.MCPStatusReady
	if err != nil {
		cp.Status.Message = fmt.Sprintf("reconcile error: %s", err.Error())
		cp.Status.Status = openmcpv1alpha1.MCPStatusNotReady
	} else {
		cp.Status.Message = ""
	}
	if cons != nil {
		cp.Status.Conditions = cons
		for _, con := range cons {
			if con.Status != openmcpv1alpha1.ComponentConditionStatusTrue {
				cp.Status.Status = openmcpv1alpha1.MCPStatusNotReady
				break
			}
		}
	}
	if inDeletion {
		cp.Status.Status = openmcpv1alpha1.MCPStatusDeleting
	}

	errs := []error{err}
	if err := r.Client.Status().Update(ctx, cp); err != nil {
		errs = append(errs, fmt.Errorf("error updating ManagedControlPlane status: %w", err))
	}

	return res, errors.Join(errs...)
}

func (r *ManagedControlPlaneController) handleCreateOrUpdate(ctx context.Context, mcp *openmcpv1alpha1.ManagedControlPlane, icfg *openmcpv1alpha1.InternalConfiguration, ns *corev1.Namespace, hadReconcileAnnotation bool) ([]openmcpv1alpha1.ManagedControlPlaneComponentCondition, ctrl.Result, error) {
	log := logging.FromContextOrPanic(ctx)

	// add finalizer and potentially project-workspace-labels, if they doesn't exist
	old := mcp.DeepCopy()
	finalizerChanged := controllerutil.AddFinalizer(mcp, openmcpv1alpha1.ManagedControlPlaneFinalizer)
	labelsChanged := false
	var nsLabels map[string]string
	if ns != nil {
		nsLabels = ns.Labels
	}
	if mcp.Labels == nil {
		mcp.Labels = map[string]string{}
	}
	if project, ok := nsLabels[openmcpv1alpha1.ProjectWorkspaceOperatorProjectLabel]; ok {
		mcp.Labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelProject] = project
		labelsChanged = true
	} else {
		if _, ok := mcp.Labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelProject]; ok {
			delete(mcp.Labels, openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelProject)
			labelsChanged = true
		}
	}
	if workspace, ok := nsLabels[openmcpv1alpha1.ProjectWorkspaceOperatorWorkspaceLabel]; ok {
		mcp.Labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelWorkspace] = workspace
		labelsChanged = true
	} else {
		if _, ok := mcp.Labels[openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelWorkspace]; ok {
			delete(mcp.Labels, openmcpv1alpha1.ManagedControlPlaneBackReferenceLabelWorkspace)
			labelsChanged = true
		}
	}
	if finalizerChanged || labelsChanged {
		log.Debug("Adding finalizer and/or project/workspace lables to ManagedControlPlane", "finalizerChanged", finalizerChanged, "labelsChanged", labelsChanged)
		if err := r.Client.Patch(ctx, mcp, client.MergeFrom(old)); err != nil {
			return nil, ctrl.Result{}, fmt.Errorf("error adding finalizer and/or project/workspace labels: %w", err)
		}
		// old = mcp.DeepCopy()
	}

	// generate internal resources and fetch status
	allCompHandlers := components.Registry.GetKnownComponents()
	curCompHandlers, err := componentutils.GetComponents[*components.ComponentHandler](components.Registry, ctx, r.Client, mcp.Name, mcp.Namespace)
	if err != nil {
		return nil, ctrl.Result{}, fmt.Errorf("error fetching current components from cluster: %w", err)
	}
	genCompHandlers, err := r.ManagedControlPlaneToSplitInternalResources(mcp, icfg, ns, r.Client.Scheme(), hadReconcileAnnotation)
	if err != nil {
		return nil, ctrl.Result{}, fmt.Errorf("unable to convert ManagedControlPlane to internal resources: %w", err)
	}
	log.Info("Generated and existing components", "generatedComponents", keyStringList(genCompHandlers, true), "existingComponents", keyStringList(curCompHandlers, true))
	allErrs := []error{}
	mcpSuccessful := true
	componentErrors := []string{}
	componentMessages := []string{}
	cpcConditions := map[string]openmcpv1alpha1.ManagedControlPlaneComponentCondition{}
	for ct := range allCompHandlers {
		clog := log.WithValues("component", string(ct))
		ch, existingOk := curCompHandlers[ct]
		genCh, generatedOk := genCompHandlers[ct]

		var cons openmcpv1alpha1.ComponentConditionList
		if existingOk {
			// collect conditions from all existing components
			cons = ch.Resource().GetCommonStatus().Conditions
		}
		if len(cons) == 0 && generatedOk {
			// if the component resource doesn't have any conditions (most probably due to just being created),
			// create the expected conditions with status 'Unknown' on the ManagedControlPlane
			cons = openmcpv1alpha1.ComponentConditionList{
				missingCondition(string(genCh.Resource().Type())),
			}
		}
		for _, con := range cons {
			if unicode.IsLower(rune(con.Type[0])) {
				// don't export conditions starting with a lowercase letter
				continue
			}
			if con.Type == ct.ReconciliationCondition() {
				if con.Status != openmcpv1alpha1.ComponentConditionStatusTrue {
					mcpSuccessful = false
					componentErrors = append(componentErrors, fmt.Sprintf("\t%s", componentErrorFromCondition(ct, con)))
				} else if con.Message != "" {
					componentMessages = append(componentMessages, fmt.Sprintf("%s: %s", string(ct), strings.ReplaceAll(con.Message, "\n", "\n\t")))
				}
				continue
			}
			if ex, ok := cpcConditions[con.Type]; ok && ex.ManagedBy != ct {
				allErrs = append(allErrs, fmt.Errorf("internal error: component '%s' has condition '%s', but that condition is already managed by component '%s'", string(ct), con.Type, string(ex.ManagedBy)))
			} else {
				cpcConditions[con.Type] = componentConditionToCPCondition(ct, con)
			}
		}

		if !existingOk && !generatedOk {
			// component is not defined in managedcontrolplane spec and there is no leftover component resource
			continue
		} else if existingOk && !generatedOk {
			// component has been deleted from managedcontrolplane spec, remove it
			if !ch.Resource().GetDeletionTimestamp().IsZero() {
				clog.Debug("Component not in ManagedControlPlane spec, resource still exists but already has deletion timestamp, nothing to do")
			} else {
				clog.Debug("Component not in ManagedControlPlane spec but resource exists in cluster, removing it")
				if err := r.Client.Delete(ctx, ch.Resource()); err != nil {
					if !apierrors.IsNotFound(err) {
						allErrs = append(allErrs, err)
					}
				}
			}
			cpGen, icGen, err := componentutils.GetCreatedFromGeneration(ch.Resource())
			if err != nil {
				clog.Error(err, "error checking for deleted resource's created-from labels, trying to patch them anyway")
			}
			if err != nil || cpGen != mcp.Generation || (icfg == nil && icGen != -1) || (icfg != nil && icGen != icfg.Generation) {
				clog.Debug("Patching outdated created-from generation labels on resource")
				if err := r.Client.Patch(ctx, ch.Resource(), componentutils.GenerateCreatedFromGenerationPatch(mcp, icfg, hadReconcileAnnotation)); err != nil {
					if !apierrors.IsNotFound(err) {
						allErrs = append(allErrs, err)
					}
				}
			}
			continue
		}
		// component needs to be either created or updated
		if ch == nil {
			clog.Debug("Creating resource for component")
			ch = allCompHandlers[ct]
			ch.Resource().SetName(genCh.Resource().GetName())
			ch.Resource().SetNamespace(genCh.Resource().GetNamespace())
		} else {
			clog.Debug("Updating resource for component")
		}
		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ch.Resource(), func() error {
			// remove potentially leftover ignore annotation
			if openmcpctrlutil.HasAnnotationWithValue(ch.Resource(), openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueIgnore) && !openmcpctrlutil.HasAnnotationWithValue(genCh.Resource(), openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueIgnore) {
				anns := ch.Resource().GetAnnotations()
				// since removing an annotation usually doesn't trigger a reconciliation, add the reconcile annotation instead
				anns[openmcpv1alpha1.OperationAnnotation] = openmcpv1alpha1.OperationAnnotationValueReconcile
				ch.Resource().SetAnnotations(anns)
			}
			ch.Resource().SetAnnotations(maps.Merge(filters.FilterMap(ch.Resource().GetAnnotations(), filters.Not(isMCPKeyFilter)), genCh.Resource().GetAnnotations()))
			ch.Resource().SetLabels(maps.Merge(filters.FilterMap(ch.Resource().GetLabels(), filters.Not(isMCPKeyFilter)), genCh.Resource().GetLabels()))
			ch.Resource().SetOwnerReferences(genCh.Resource().GetOwnerReferences())
			if err := ch.Resource().SetSpec(genCh.Resource().GetSpec()); err != nil {
				return fmt.Errorf("internal error transferring generated spec to existing resource for component '%s': %w", string(ct), err)
			}
			return nil
		}); err != nil {
			allErrs = append(allErrs, fmt.Errorf("error creating/updating component resource for component '%s': %w", string(ct), err))
		}

		if err := ch.Converter().InjectStatus(ch.Resource().GetExternalStatus(), &mcp.Status); err != nil {
			allErrs = append(allErrs, fmt.Errorf("internal error transferring status of component '%s' into ManagedControlPlane: %w", string(ct), err))
		}

	}

	slices.Sort(componentMessages)
	var oldMCPSuccessfulCon *openmcpv1alpha1.ManagedControlPlaneComponentCondition
	if len(mcp.Status.Conditions) > 0 {
		for i := len(mcp.Status.Conditions) - 1; i >= 0; i-- {
			// the sorting usually puts the MCPSuccessful condition last, so let's search for it from the end of the list
			if mcp.Status.Conditions[i].Type == cconst.ConditionMCPSuccessful {
				oldMCPSuccessfulCon = mcp.Status.Conditions[i].DeepCopy()
				break
			}
		}
	}
	var mcpSuccessfulCon openmcpv1alpha1.ComponentCondition
	if oldMCPSuccessfulCon == nil {
		// MCPSuccessful condition not found, create a new one
		mcpSuccessfulCon = componentutils.NewCondition(cconst.ConditionMCPSuccessful, openmcpv1alpha1.ComponentConditionStatusFromBool(mcpSuccessful), cconst.ReasonAllComponentsReconciledSuccessfully, strings.Join(componentMessages, "\n"))
	} else {
		// update the existing MCPSuccessful condition to keep the lastTransitionTimestamp intact
		mcpSuccessfulCon = componentutils.ConditionUpdater([]openmcpv1alpha1.ComponentCondition{oldMCPSuccessfulCon.ComponentCondition}, false).UpdateCondition(cconst.ConditionMCPSuccessful, openmcpv1alpha1.ComponentConditionStatusFromBool(mcpSuccessful), cconst.ReasonAllComponentsReconciledSuccessfully, strings.Join(componentMessages, "\n")).Conditions()[0]
	}
	if !mcpSuccessful {
		slices.Sort(componentErrors)
		mcpSuccessfulCon.Reason = cconst.ReasonNotAllComponentsReconciledSuccessfully
		mcpSuccessfulCon.Message = fmt.Sprintf("The following components could not be reconciled successfully:\n%s", strings.Join(componentErrors, "\n"))
	}
	return append(sortConditions(cpcConditions), openmcpv1alpha1.ManagedControlPlaneComponentCondition{ComponentCondition: mcpSuccessfulCon}), ctrl.Result{}, errors.Join(allErrs...)
}

func (r *ManagedControlPlaneController) handleDelete(ctx context.Context, mcp *openmcpv1alpha1.ManagedControlPlane, _ *corev1.Namespace, hadReconcileAnnotation bool) ([]openmcpv1alpha1.ManagedControlPlaneComponentCondition, ctrl.Result, error) {
	// get all component resources
	log := logging.FromContextOrPanic(ctx)
	compHandlers, err := componentutils.GetComponents[*components.ComponentHandler](components.Registry, ctx, r.Client, mcp.Name, mcp.Namespace)
	if err != nil {
		return nil, ctrl.Result{}, fmt.Errorf("error fetching current components from cluster: %w", err)
	}

	if len(compHandlers) == 0 {
		// all components have been successfully deleted
		log.Info("All components have been deleted")
		old := mcp.DeepCopy()
		changed := controllerutil.RemoveFinalizer(mcp, openmcpv1alpha1.ManagedControlPlaneFinalizer)
		if changed {
			if err := r.Client.Patch(ctx, mcp, client.MergeFrom(old)); err != nil {
				return nil, ctrl.Result{}, fmt.Errorf("error removing finalizer from ManagedControlPlane: %w", err)
			}
		}
		return nil, ctrl.Result{}, nil
	}

	log.Info("Deleting remaining components", "existingComponents", keyStringList(compHandlers, true))
	allErrs := []error{}
	mcpSuccessful := true
	componentErrors := []string{}
	componentMessages := []string{}
	cpcConditions := map[string]openmcpv1alpha1.ManagedControlPlaneComponentCondition{}
	for ct, ch := range compHandlers {
		// delete components
		if err := r.Client.Delete(ctx, ch.Resource()); err != nil {
			allErrs = append(allErrs, fmt.Errorf("error deleting resource for component '%s': %w", string(ct), err))
		}
		if hadReconcileAnnotation {
			if err := componentutils.PatchAnnotation(ctx, r.Client, ch.Resource(), openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueReconcile); err != nil && !componentutils.IsAnnotationAlreadyExistsError(err) {
				allErrs = append(allErrs, fmt.Errorf("error patching reconcile operation annotation on resource for component '%s': %w", string(ct), err))
			}
		}

		for _, con := range ch.Resource().GetCommonStatus().Conditions {
			if unicode.IsLower(rune(con.Type[0])) {
				// don't export conditions starting with a lowercase letter
				continue
			}
			if con.Type == ct.ReconciliationCondition() {
				if con.Status != openmcpv1alpha1.ComponentConditionStatusTrue {
					mcpSuccessful = false
					componentErrors = append(componentErrors, fmt.Sprintf("\t%s", componentErrorFromCondition(ct, con)))
				} else if con.Message != "" {
					componentMessages = append(componentMessages, fmt.Sprintf("%s: %s", string(ct), strings.ReplaceAll(con.Message, "\n", "\n\t")))
				}
				continue
			}
			if ex, ok := cpcConditions[con.Type]; ok && ex.ManagedBy != ct {
				allErrs = append(allErrs, fmt.Errorf("internal error: component '%s' has condition '%s', but that condition is already managed by component '%s'", string(ct), con.Type, string(ex.ManagedBy)))
			} else {
				cpcConditions[con.Type] = componentConditionToCPCondition(ct, con)
			}
		}

		if err := ch.Converter().InjectStatus(ch.Resource().GetExternalStatus(), &mcp.Status); err != nil {
			allErrs = append(allErrs, fmt.Errorf("internal error transferring status of component '%s' into ManagedControlPlane: %w", string(ct), err))
		}
	}

	slices.Sort(componentMessages)
	mcpReadyCon := componentutils.NewCondition(cconst.ConditionMCPSuccessful, openmcpv1alpha1.ComponentConditionStatusFromBool(mcpSuccessful), cconst.ReasonAllComponentsReconciledSuccessfully, strings.Join(componentMessages, "\n"))
	if !mcpSuccessful {
		slices.Sort(componentErrors)
		mcpReadyCon.Reason = cconst.ReasonNotAllComponentsReconciledSuccessfully
		mcpReadyCon.Message = fmt.Sprintf("The following components could not be reconciled successfully:\n%s", strings.Join(componentErrors, "\n"))
	}
	return append(sortConditions(cpcConditions), openmcpv1alpha1.ManagedControlPlaneComponentCondition{ComponentCondition: mcpReadyCon}), ctrl.Result{}, errors.Join(allErrs...)
}

// keyStringList returns the keys of the given map as list.
// Uses fmt.Sprint to convert the key into a string.
// If sort is true, the returned slice is sorted lexically.
func keyStringList[K comparable, V any](source map[K]V, sort bool) []string {
	res := make([]string, 0, len(source))
	for k := range source {
		res = append(res, fmt.Sprint(k))
	}
	if sort {
		slices.Sort(res)
	}
	return res
}

// valueList returns the values of a map as a slice.
func valueList[K comparable, V any](source map[K]V) []V {
	res := make([]V, 0, len(source))
	for _, v := range source {
		res = append(res, v)
	}
	return res
}

// componentConditionToCPCondition converts component conditions into ManagedControlPlane conditions by adding the managing component.
func componentConditionToCPCondition(ct openmcpv1alpha1.ComponentType, con openmcpv1alpha1.ComponentCondition) openmcpv1alpha1.ManagedControlPlaneComponentCondition {
	return openmcpv1alpha1.ManagedControlPlaneComponentCondition{
		ComponentCondition: con,
		ManagedBy:          ct,
	}
}

// missingCondition is used to create a condition with status 'Unknown' when a component's resource does not contain any conditions.
func missingCondition(conType string) openmcpv1alpha1.ComponentCondition {
	return openmcpv1alpha1.ComponentCondition{
		Type:    conType,
		Status:  openmcpv1alpha1.ComponentConditionStatusUnknown,
		Reason:  cconst.ReasonNoConditions,
		Message: "This component does not expose any conditions.",
	}
}

// sortConditions takes a map of CPC conditions, extracts the values into a slice and returns the slice after sorting it.
// It is sorted lexically by the ManagedBy field first and the Type field second.
func sortConditions(mappedCons map[string]openmcpv1alpha1.ManagedControlPlaneComponentCondition) []openmcpv1alpha1.ManagedControlPlaneComponentCondition {
	cons := valueList(mappedCons)
	slices.SortFunc(cons, func(a, b openmcpv1alpha1.ManagedControlPlaneComponentCondition) int {
		res := strings.Compare(string(a.ManagedBy), string(b.ManagedBy))
		if res == 0 {
			res = strings.Compare(a.Type, b.Type)
		}
		return res
	})
	return cons
}

// componentErrorFromCondition creates a one-liner error message from a component condition.
func componentErrorFromCondition(ct openmcpv1alpha1.ComponentType, con openmcpv1alpha1.ComponentCondition) string {
	return fmt.Sprintf("%s: [%s] %s", string(ct), con.Reason, con.Message)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedControlPlaneController) SetupWithManager(mgr ctrl.Manager) error {
	ctrlbuild := ctrl.NewControllerManagedBy(mgr).For(&openmcpv1alpha1.ManagedControlPlane{}, builder.WithPredicates(predicate.Or(
		predicate.GenerationChangedPredicate{},
		predicate.LabelChangedPredicate{},
		openmcpctrlutil.GotAnnotationPredicate(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueReconcile),
		openmcpctrlutil.LostAnnotationPredicate(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueIgnore),
	)))
	ctrlbuild.Owns(&openmcpv1alpha1.InternalConfiguration{}, builder.WithPredicates(predicate.GenerationChangedPredicate{}))
	for _, ch := range components.Registry.GetKnownComponents() {
		ctrlbuild.Owns(ch.Resource(), builder.WithPredicates(componentutils.StatusChangedPredicate{}))
	}
	return ctrlbuild.Complete(r)
}
