package config

import (
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

const (
	DefaultSystemIdPName       = "openmcp"
	DefaultSystemUsernameClaim = "email"
	DefaultSystemGroupsClaim   = "groups"

	DefaultCratedIdPName      = "crate"
	DefaultCrateClientID      = "mcp"
	DefaultCrateUsernameClaim = "sub"
)

// AuthenticationConfig contains the configuration for the authentication controller.
type AuthenticationConfig struct {
	// SystemIdentityProvider contains the configuration for the system identity provider.
	SystemIdentityProvider v1alpha1.IdentityProvider `json:"systemIdentityProvider,omitempty"`
	// CrateIdentityProvider contains the configuration for the Crate token issuer.
	// This can be used to validate tokens issued by the crate cluster.
	// +optional
	CrateIdentityProvider *v1alpha1.IdentityProvider `json:"crateIdentityProvider,omitempty"`
}

// SetDefaults sets the default values for the authentication configuration when not set.
func (ac *AuthenticationConfig) SetDefaults() {
	// SystemIdentityProvider
	if ac.SystemIdentityProvider.Name == "" {
		ac.SystemIdentityProvider.Name = DefaultSystemIdPName
	}

	if ac.SystemIdentityProvider.UsernameClaim == "" {
		ac.SystemIdentityProvider.UsernameClaim = DefaultSystemUsernameClaim
	}

	if ac.SystemIdentityProvider.GroupsClaim == "" {
		ac.SystemIdentityProvider.GroupsClaim = DefaultSystemGroupsClaim
	}

	// CrateIdentityProvider
	if ac.CrateIdentityProvider != nil {
		if ac.CrateIdentityProvider.Name == "" {
			ac.CrateIdentityProvider.Name = DefaultCratedIdPName
		}

		if ac.CrateIdentityProvider.ClientID == "" {
			ac.CrateIdentityProvider.ClientID = DefaultCrateClientID
		}

		if ac.CrateIdentityProvider.UsernameClaim == "" {
			ac.CrateIdentityProvider.UsernameClaim = DefaultCrateUsernameClaim
		}
	}
}

// Validate validates the authentication configuration.
func Validate(ac *AuthenticationConfig) error {
	errs := field.ErrorList{}
	errs = append(errs, v1alpha1.ValidateIdp(ac.SystemIdentityProvider, field.NewPath("systemIdentityProvider"))...)
	if ac.CrateIdentityProvider != nil {
		errs = append(errs, v1alpha1.ValidateIdp(*ac.CrateIdentityProvider, field.NewPath("crateIdentityProvider"))...)
	}
	return errs.ToAggregate()
}
