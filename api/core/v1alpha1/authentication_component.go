package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/sets"

	openmcperrors "github.tools.sap/CoLa/mcp-operator/api/errors"
)

const AuthenticationComponent ComponentType = "Authentication"

// Type returns the type of the Authentication component.
func (*Authentication) Type() ComponentType {
	return AuthenticationComponent
}

// GetSpec returns the spec of the Authentication component.
func (a *Authentication) GetSpec() any {
	return &a.Spec
}

// SetSpec sets the spec of the Authentication component.
func (a *Authentication) SetSpec(cfg any) error {
	as, ok := cfg.(*AuthenticationSpec)
	if !ok {
		return openmcperrors.ErrWrongComponentConfigType
	}
	a.Spec = *as
	return nil
}

// GetCommonStatus returns the common status of the Authentication component.
func (a *Authentication) GetCommonStatus() CommonComponentStatus {
	return a.Status.CommonComponentStatus
}

// SetCommonStatus sets the common status of the Authentication component.
func (a *Authentication) SetCommonStatus(status CommonComponentStatus) {
	a.Status.CommonComponentStatus = status
}

// GetExternalStatus returns the external status of the Authentication component.
func (a *Authentication) GetExternalStatus() any {
	return a.Status.ExternalAuthenticationStatus
}

// GetRequiredConditions implements Component.
func (a *Authentication) GetRequiredConditions() sets.Set[string] {
	// ToDo: Compute a more precise set of expected conditions based on the spec instead of returning a static set.
	return sets.New(a.Type().HealthyCondition(), a.Type().ReconciliationCondition())
}
