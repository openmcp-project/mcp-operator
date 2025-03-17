package v1alpha1

// CommonConfig contains configuration that is shared between multiple components.
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.desiredRegion)|| has(self.desiredRegion)",message="desiredRegion is required once set"
type CommonConfig struct {
	// DesiredRegion allows customers to specify a desired region proximity.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="RegionSpecification is immutable"
	DesiredRegion *RegionSpecification `json:"desiredRegion,omitempty"`
}

// InternalCommonConfig contains internal configuration that is shared between multiple components.
type InternalCommonConfig struct {
}
