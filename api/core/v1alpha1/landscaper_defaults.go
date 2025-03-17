package v1alpha1

// Default sets defaults.
// This modifies the receiver object.
// Note that only the parts which belong to the configured type are defaulted, everything else is ignored.
func (lss *LandscaperSpec) Default() {
	if lss.Deployers == nil {
		lss.Deployers = []string{
			"helm",
			"manifest",
			"container",
		}
	}
}

// Validate validates the configuration.
// Only the configuration that belongs to the configured type is validated, configuration for other types is ignored.
func (lss *LandscaperSpec) Validate(path string, morePaths ...string) error {
	return nil
}
