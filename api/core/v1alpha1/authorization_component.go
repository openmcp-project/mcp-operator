package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/sets"

	openmcperrors "github.tools.sap/CoLa/mcp-operator/api/errors"
)

const AuthorizationComponent ComponentType = "Authorization"

// Type returns the type of the Authentication component.
func (*Authorization) Type() ComponentType {
	return AuthorizationComponent
}

// GetSpec returns the spec of the Authentication component.
func (a *Authorization) GetSpec() any {
	return &a.Spec
}

// SetSpec sets the spec of the Authentication component.
func (a *Authorization) SetSpec(cfg any) error {
	as, ok := cfg.(*AuthorizationSpec)
	if !ok {
		return openmcperrors.ErrWrongComponentConfigType
	}
	a.Spec = *as
	return nil
}

// GetCommonStatus returns the common status of the Authentication component.
func (a *Authorization) GetCommonStatus() CommonComponentStatus {
	return a.Status.CommonComponentStatus
}

// SetCommonStatus sets the common status of the Authentication component.
func (a *Authorization) SetCommonStatus(status CommonComponentStatus) {
	a.Status.CommonComponentStatus = status
}

// GetExternalStatus returns the external status of the Authentication component.
func (a *Authorization) GetExternalStatus() any {
	return a.Status.ExternalAuthorizationStatus
}

// GetRequiredConditions implements Component.
func (a *Authorization) GetRequiredConditions() sets.Set[string] {
	// ToDo: Compute a more precise set of expected conditions based on the spec instead of returning a static set.
	return sets.New(a.Type().HealthyCondition(), a.Type().ReconciliationCondition())
}
