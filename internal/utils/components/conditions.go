package components

import (
	"slices"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openmcp-project/mcp-operator/internal/components"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

// ComponentConditionListUpdater is a helper struct for updating a component's conditions.
// Use the ConditionUpdater constructor for initializing.
type ComponentConditionListUpdater struct {
	Now        metav1.Time
	conditions map[string]openmcpv1alpha1.ComponentCondition
	updated    sets.Set[string]
}

// ConditionUpdater creates a builder-like helper struct for updating a list of ComponentConditions.
// The 'conditions' argument contains the old condition list.
// If removeUntouched is true, the condition list returned with Conditions() will have all conditions removed that have not been updated.
// If false, all conditions will be kept.
// Note that calling this function stores the current time as timestamp that is used as LastTransitionTime if a condition's status changed.
// To overwrite this timestamp, modify the 'Now' field of the returned struct manually.
//
// The given condition list is not modified.
//
// Usage example:
// status.conditions = ConditionUpdater(status.conditions, true).UpdateCondition(...).UpdateCondition(...).Conditions()
func ConditionUpdater(conditions openmcpv1alpha1.ComponentConditionList, removeUntouched bool) *ComponentConditionListUpdater {
	res := &ComponentConditionListUpdater{
		Now:        metav1.Now(),
		conditions: make(map[string]openmcpv1alpha1.ComponentCondition, len(conditions)),
	}
	for _, con := range conditions {
		res.conditions[con.Type] = con
	}
	if removeUntouched {
		res.updated = sets.New[string]()
	}
	return res
}

// UpdateCondition updates or creates the condition with the specified type.
// All fields of the condition are updated with the values given in the arguments, but the condition's LastTransitionTime is only updated (with the timestamp contained in the receiver struct) if the status changed.
// Returns the receiver for easy chaining.
func (c *ComponentConditionListUpdater) UpdateCondition(conType string, status openmcpv1alpha1.ComponentConditionStatus, reason, message string) *ComponentConditionListUpdater {
	con := openmcpv1alpha1.ComponentCondition{
		Type:               conType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: c.Now,
	}
	old, ok := c.conditions[conType]
	if ok && old.Status == con.Status {
		// update LastTransitionTime only if status changed
		con.LastTransitionTime = old.LastTransitionTime
	}
	c.conditions[conType] = con
	if c.updated != nil {
		c.updated.Insert(conType)
	}
	return c
}

// UpdateConditionFromTemplate is a convenience wrapper around UpdateCondition which allows it to be called with a preconstructed ComponentCondition.
func (c *ComponentConditionListUpdater) UpdateConditionFromTemplate(con openmcpv1alpha1.ComponentCondition) *ComponentConditionListUpdater {
	return c.UpdateCondition(con.Type, con.Status, con.Reason, con.Message)
}

// HasCondition returns true if a condition with the given type exists in the updated condition list.
func (c *ComponentConditionListUpdater) HasCondition(conType string) bool {
	_, ok := c.conditions[conType]
	return ok && (c.updated == nil || c.updated.Has(conType))
}

// Conditions returns the updated condition list.
// If the condition updater was initialized with removeUntouched=true, this list will only contain the conditions which have been updated
// in between the condition updater creation and this method call. Otherwise, it will potentially also contain old conditions.
// The conditions are returned sorted by their type.
func (c *ComponentConditionListUpdater) Conditions() openmcpv1alpha1.ComponentConditionList {
	res := openmcpv1alpha1.ComponentConditionList{}
	for _, con := range c.conditions {
		if c.updated == nil || c.updated.Has(con.Type) {
			res = append(res, con)
		}
	}
	slices.SortStableFunc(res, func(a, b openmcpv1alpha1.ComponentCondition) int {
		return strings.Compare(a.Type, b.Type)
	})
	return res
}

// GetCondition returns a pointer to the condition for the given type, if it exists.
// Otherwise, nil is returned.
func GetCondition(ccl openmcpv1alpha1.ComponentConditionList, t string) *openmcpv1alpha1.ComponentCondition {
	for i := range ccl {
		if ccl[i].Type == t {
			return &ccl[i]
		}
	}
	return nil
}

// NewCondition creates a new ComponentCondition with the given values and the current time as LastTransitionTime.
func NewCondition(conType string, status openmcpv1alpha1.ComponentConditionStatus, reason, message string) openmcpv1alpha1.ComponentCondition {
	return openmcpv1alpha1.ComponentCondition{
		Type:               conType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}
}

// IsComponentReady returns true if the component's observedGenerations are up-to-date and all of its relevant conditions are "True".
// If relevantConditions is empty, all of the component's conditions are deemed relevant.
// Condition types in relevantConditions for which no condition exists on the component are considered "Unknown" and cause the method to return false.
func IsComponentReady(comp components.Component, relevantConditions ...string) bool {
	if comp == nil {
		return false
	}
	cpGen, icGen, err := GetCreatedFromGeneration(comp)
	if err != nil {
		return false
	}
	cs := comp.GetCommonStatus()
	cons := []openmcpv1alpha1.ComponentCondition{}
	if len(relevantConditions) == 0 {
		cons = cs.Conditions
	} else {
		for _, rc := range relevantConditions {
			found := false
			for _, con := range cs.Conditions {
				if con.Type == rc {
					cons = append(cons, con)
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	return IsComponentReadyRaw(cpGen, icGen, comp.GetGeneration(), cs.ObservedGenerations, cons...)
}

// IsComponentReadyRaw returns true if the component's observed generations are up-to-date and all of the given conditions are "True".
// The first three arguments contain the generations of the ManagedControlPlane, InternalConfiguration, and component resource, respectively.
// They are compared to the observedGenerations in the given ObservedGenerations struct (which usually comes from the component resource's status).
// The generation of the InternalConfiguration is expected to be -1, if no InternalConfiguration exists.
// The generation of the component resource can be set to -1 if it is not known (it will then be ignored).
func IsComponentReadyRaw(cpGen, icGen, rGen int64, obsGen openmcpv1alpha1.ObservedGenerations, conditions ...openmcpv1alpha1.ComponentCondition) bool {
	if !(obsGen.ManagedControlPlane == cpGen && obsGen.InternalConfiguration == icGen && (rGen < 0 || obsGen.Resource == rGen)) {
		return false
	}
	for _, con := range conditions {
		if con.Status != openmcpv1alpha1.ComponentConditionStatusTrue {
			return false
		}
	}
	return true
}

// AllConditionsTrue returns true if all given conditions are "True".
func AllConditionsTrue(conditions ...openmcpv1alpha1.ComponentCondition) bool {
	for _, con := range conditions {
		if con.Status != openmcpv1alpha1.ComponentConditionStatusTrue {
			return false
		}
	}
	return true
}
