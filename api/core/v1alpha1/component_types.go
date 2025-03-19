// +kubebuilder:object:generate=true
package v1alpha1

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ComponentType string

const (
	ComponentTypeUndefined ComponentType = "Undefined"
)

type ComponentConditionStatus string

const (
	// ComponentConditionStatusUnknown represents an unknown status for the condition.
	ComponentConditionStatusUnknown ComponentConditionStatus = "Unknown"
	// ComponentConditionStatusTrue marks the condition as true.
	ComponentConditionStatusTrue ComponentConditionStatus = "True"
	// ComponentConditionStatusFalse marks the condition as false.
	ComponentConditionStatusFalse ComponentConditionStatus = "False"
)

type ComponentCondition struct {
	// Type is the type of the condition.
	// This is a unique identifier and each type of condition is expected to be managed by exactly one component controller.
	Type string `json:"type"`

	// Status is the status of the condition.
	Status ComponentConditionStatus `json:"status"`

	// Reason is expected to contain a CamelCased string that provides further information regarding the condition.
	// It should have a fixed value set (like an enum) to be machine-readable. The value set depends on the condition type.
	// It is optional, but should be filled at least when Status is not "True".
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message contains further details regarding the condition.
	// It is meant for human users, Reason should be used for programmatic evaluation instead.
	// It is optional, but should be filled at least when Status is not "True".
	// +optional
	Message string `json:"message,omitempty"`

	// LastTransitionTime specifies the time when this condition's status last changed.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

type ObservedGenerations struct {
	// Resource contains the last generation of this resource that has been handled by the controller.
	// This refers to metadata.generation of this resource.
	Resource int64 `json:"resource"`

	// ManagedControlPlane contains the last generation of the owning v1alpha1.ManagedControlPlane that has been by the controller.
	// Note that the component's controller does not read the ManagedControlPlane resource itself, but fetches this information from a label which is populated by the v1alpha1.ManagedControlPlane controller.
	// This refers to metadata.generation of the owning v1alpha1.ManagedControlPlane resource.
	// This value is probably identical to the one in 'Resource', unless something else than the v1alpha1.ManagedControlPlane controller touched the spec of this resource.
	ManagedControlPlane int64 `json:"managedControlPlane"`

	// InternalConfiguration contains the last generation of the InternalConfiguration belonging to the owning v1alpha1.ManagedControlPlane that has been seen by the controller.
	// Note that the component's controller does not read the InternalConfiguration itself, but fetches this information from a label which is populated by the v1alpha1.ManagedControlPlane controller.
	// This refers to metadata.generation of the InternalConfiguration belonging to the owning v1alpha1.ManagedControlPlane, if any.
	// If the resource does not have a label containing the generation of the corresponding InternalConfiguration, this means that no InternalConfiguration exists for
	// the owning v1alpha1.ManagedControlPlane. In that case, the value of this field is expected to be -1.
	InternalConfiguration int64 `json:"internalConfiguration"`
}

// CommonComponentStatus contains fields which all component resources' statuses must contain.
type CommonComponentStatus struct {
	// Conditions containts the conditions of the component.
	// For each component, this is expected to contain at least one condition per top-level node that component has in the ManagedControlPlane's spec.
	// This condition is expected to be named "<node>Healthy" and to describe the general availability of the functionality configured by that top-level node.
	Conditions ComponentConditionList `json:"conditions,omitempty"`

	// ObservedGenerations contains information about the observed generations of a component.
	// This information is required to determine whether a component's controller has already processed some changes or not.
	ObservedGenerations ObservedGenerations `json:"observedGenerations,omitempty"`
}

// ComponentConditionList is a list of ComponentConditions.
type ComponentConditionList []ComponentCondition

// ComponentConditionStatusFromBoolPtr converts a bool pointer into the corresponding ComponentConditionStatus.
// If nil, "Unknown" is returned.
func ComponentConditionStatusFromBoolPtr(src *bool) ComponentConditionStatus {
	if src == nil {
		return ComponentConditionStatusUnknown
	}
	return ComponentConditionStatusFromBool(*src)
}

// ComponentConditionStatusFromBool converts a bool into the corresponding ComponentConditionStatus.
func ComponentConditionStatusFromBool(src bool) ComponentConditionStatus {
	if src {
		return ComponentConditionStatusTrue
	}
	return ComponentConditionStatusFalse
}

// IsTrue returns true if the ComponentCondition's status is "True".
// Note that the status can be "Unknown", so !IsTrue() is not the same as IsFalse().
func (cc ComponentCondition) IsTrue() bool {
	return cc.Status == ComponentConditionStatusTrue
}

// IsFalse returns true if the ComponentCondition's status is "False".
// Note that the status can be "Unknown", so !IsFalse() is not the same as IsTrue().
func (cc ComponentCondition) IsFalse() bool {
	return cc.Status == ComponentConditionStatusFalse
}

// IsUnknown returns true if the ComponentCondition's status is "Unknown".
func (cc ComponentCondition) IsUnknown() bool {
	return cc.Status == ComponentConditionStatusUnknown
}

// Finalizer returns the finalizer this component sets on its own resources.
func (ct ComponentType) Finalizer() string {
	return fmt.Sprintf("%s.%s", strings.ToLower(string(ct)), BaseDomain)
}

// DependencyFinalizer returns the finalizer this component uses to mark its dependencies.
func (ct ComponentType) DependencyFinalizer() string {
	return fmt.Sprintf("%s%s", DependencyFinalizerPrefix, strings.ToLower(string(ct)))
}

// ReconciliationCondition returns the name of the condition that holds the information whether the last
// reconciliation of the component was successful or not.
// It resolves to "<componentType>Reconciliation".
func (ct ComponentType) ReconciliationCondition() string {
	return fmt.Sprintf("%sReconciliation", string(ct))
}

// HealthyCondition returns the name of the condition that holds the information whether the component is healthy or not.
// It resolves to "<componentType>Healthy".
func (ct ComponentType) HealthyCondition() string {
	return fmt.Sprintf("%sHealthy", string(ct))
}
