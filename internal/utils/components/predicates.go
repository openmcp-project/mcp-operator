package components

// This package contains predicates which can be used for constructing controllers.

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	colactrlutil "github.tools.sap/CoLa/controller-utils/pkg/controller"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

// DefaultComponentControllerPredicates returns a predicate combination which should be useful for most - if not all - component controllers.
func DefaultComponentControllerPredicates() predicate.Predicate {
	return predicate.And(
		predicate.Or(
			predicate.GenerationChangedPredicate{},
			colactrlutil.GotAnnotationPredicate(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueReconcile),
			colactrlutil.LostAnnotationPredicate(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueIgnore),
			GenerationLabelsChangedPredicate{},
		),
		predicate.Not(
			colactrlutil.HasAnnotationPredicate(openmcpv1alpha1.OperationAnnotation, openmcpv1alpha1.OperationAnnotationValueIgnore),
		),
	)
}

// GenerationLabelsChangedPredicate reacts on changes to the cp/ir generation labels.
type GenerationLabelsChangedPredicate struct {
	predicate.Funcs
}

var _ predicate.Predicate = GenerationLabelsChangedPredicate{}

func (GenerationLabelsChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}

	// old labels
	// an error means that either the CP generation label did not exist or is not an int, we can ignore the former and can't do anything about the latter case here
	oldCPGen, oldIRGen, _ := GetCreatedFromGeneration(e.ObjectOld)
	newCPGen, newIRGen, _ := GetCreatedFromGeneration(e.ObjectNew)
	return oldCPGen != newCPGen || oldIRGen != newIRGen
}

var _ predicate.Predicate = StatusChangedPredicate{}

// StatusChangedPredicate returns true if the object's status changed.
// Getting the status is done via reflection and only works if the corresponding field is named 'Status'.
// If getting the status fails, this predicate always returns true.
type StatusChangedPredicate struct {
	predicate.Funcs
}

func (p StatusChangedPredicate) Update(e event.UpdateEvent) bool {
	oldStatus := getStatus(e.ObjectOld)
	newStatus := getStatus(e.ObjectNew)
	if oldStatus == nil || newStatus == nil {
		return true
	}
	return !reflect.DeepEqual(oldStatus, newStatus)
}

func getStatus(obj any) any {
	if obj == nil {
		return nil
	}
	val := reflect.ValueOf(obj).Elem()
	for i := 0; i < val.NumField(); i++ {
		if val.Type().Field(i).Name == "Status" {
			return val.Field(i).Interface()
		}
	}
	return nil
}
