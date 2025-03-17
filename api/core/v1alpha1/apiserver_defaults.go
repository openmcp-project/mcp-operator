package v1alpha1

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Default sets defaults.
// This modifies the receiver object.
// Note that only the parts which belong to the configured type are defaulted, everything else is ignored.
func (asSpec *APIServerSpec) Default() {
	switch asSpec.Type {
	case Gardener:
		if asSpec.GardenerConfig == nil {
			asSpec.GardenerConfig = &GardenerConfiguration{}
		}
		asSpec.GardenerConfig.Default()
	}
}

// Validate validates the configuration.
// Only the configuration that belongs to the configured type is validated, configuration for other types is ignored.
func (asSpec *APIServerSpec) Validate(path string, morePaths ...string) error {
	allErrs := field.ErrorList{}
	fldPath := field.NewPath(path, morePaths...)

	switch asSpec.Type {
	case Gardener, GardenerDedicated:
		allErrs = append(allErrs, asSpec.GardenerConfig.Validate(fldPath.Child("gardener"))...)
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), string(asSpec.Type), []string{string(Gardener), string(GardenerDedicated)}))
	}

	return allErrs.ToAggregate()
}
