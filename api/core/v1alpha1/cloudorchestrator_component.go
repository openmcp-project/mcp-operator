package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/sets"

	openmcperrors "github.tools.sap/CoLa/mcp-operator/api/errors"
)

const CloudOrchestratorComponent ComponentType = "CloudOrchestrator"

// Type implements Component.
func (*CloudOrchestrator) Type() ComponentType {
	return CloudOrchestratorComponent
}

// GetSpec implements Component.
func (o *CloudOrchestrator) GetSpec() any {
	return &o.Spec
}

// SetSpec implements Component.
func (o *CloudOrchestrator) SetSpec(cfg any) error {
	cSpec, ok := cfg.(*CloudOrchestratorSpec)
	if !ok {
		return openmcperrors.ErrWrongComponentConfigType
	}
	o.Spec = *cSpec
	return nil
}

// GetCommonStatus implements Component.
func (o *CloudOrchestrator) GetCommonStatus() CommonComponentStatus {
	return o.Status.CommonComponentStatus
}

// SetCommonStatus implements Component.
func (o *CloudOrchestrator) SetCommonStatus(status CommonComponentStatus) {
	o.Status.CommonComponentStatus = status
}

// GetExternalStatus implements Component.
func (o *CloudOrchestrator) GetExternalStatus() any {
	return o.Status.ExternalCloudOrchestratorStatus
}

// GetRequiredConditions implements Component.
func (o *CloudOrchestrator) GetRequiredConditions() sets.Set[string] {
	// ToDo: Compute a more precise set of expected conditions based on the spec instead of returning a static set.
	return sets.New(o.Type().HealthyCondition(), o.Type().ReconciliationCondition())
}
