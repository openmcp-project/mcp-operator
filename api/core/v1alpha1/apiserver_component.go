package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/sets"

	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
)

const APIServerComponent ComponentType = "APIServer"
const ConditionAPIServerHealthy = "apiServerHealthy"

// Type implements Component.
func (*APIServer) Type() ComponentType {
	return APIServerComponent
}

// GetHealthCondition implements Component.
func (*APIServer) GetHealthCondition() string {
	return ConditionAPIServerHealthy
}

// GetSpec implements Component.
func (as *APIServer) GetSpec() any {
	return &as.Spec
}

// SetSpec implements Component.
func (as *APIServer) SetSpec(cfg any) error {
	apiServerSpec, ok := cfg.(*APIServerSpec)
	if !ok {
		return openmcperrors.ErrWrongComponentConfigType
	}
	as.Spec = *apiServerSpec
	return nil
}

// GetCommonStatus implements Component.
func (as *APIServer) GetCommonStatus() CommonComponentStatus {
	return as.Status.CommonComponentStatus
}

// SetCommonStatus implements Component.
func (as *APIServer) SetCommonStatus(status CommonComponentStatus) {
	as.Status.CommonComponentStatus = status
}

// GetExternalStatus implements Component.
func (as *APIServer) GetExternalStatus() any {
	return as.Status.ExternalAPIServerStatus
}

// GetRequiredConditions implements Component.
func (as *APIServer) GetRequiredConditions() sets.Set[string] {
	// ToDo: Compute a more precise set of expected conditions based on the spec instead of returning a static set.
	return sets.New(as.Type().HealthyCondition(), as.Type().ReconciliationCondition())
}
