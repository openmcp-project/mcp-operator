package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/sets"

	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
)

const LandscaperComponent ComponentType = "Landscaper"

// Type implements Component.
func (*Landscaper) Type() ComponentType {
	return LandscaperComponent
}

// GetSpec implements Component.
func (ls *Landscaper) GetSpec() any {
	return &ls.Spec
}

// SetSpec implements Component.
func (ls *Landscaper) SetSpec(cfg any) error {
	lsSpec, ok := cfg.(*LandscaperSpec)
	if !ok {
		return openmcperrors.ErrWrongComponentConfigType
	}
	ls.Spec = *lsSpec
	return nil
}

func (ls *Landscaper) GetCommonStatus() CommonComponentStatus {
	return ls.Status.CommonComponentStatus
}

// SetCommonStatus implements Component.
func (ls *Landscaper) SetCommonStatus(status CommonComponentStatus) {
	ls.Status.CommonComponentStatus = status
}

// GetExternalStatus implements Component.
func (ls *Landscaper) GetExternalStatus() any {
	return ls.Status.ExternalLandscaperStatus
}

// GetRequiredConditions implements Component.
func (ls *Landscaper) GetRequiredConditions() sets.Set[string] {
	// ToDo: Compute a more precise set of expected conditions based on the spec instead of returning a static set.
	return sets.New(ls.Type().HealthyCondition(), ls.Type().ReconciliationCondition())
}
