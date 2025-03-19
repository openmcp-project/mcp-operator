package config

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

// APIServerProviderConfiguration contains the configuration for the APIServer provider.
// This is everything which must be configured, should neither be a command-line argument
// nor be configurable in the ManagedControlPlane resource.
// For example: Garden cluster kubeconfig and project name for creating Gardener shoots.
type APIServerProviderConfiguration struct {
	CommonConfig `json:",inline"`

	// GardenerConfig contains configuration for creating shoot clusters on a Gardener landscape.
	GardenerConfig *MultiGardenerConfiguration `json:"gardener,omitempty"`
}

type CommonConfig struct {
	// ServiceAccountNamespace is the namespace on the API Server to be used for serviceaccounts.
	// Defaults to the cola system namespace (usually 'openmcp-system').
	ServiceAccountNamespace *string `json:"serviceAccountNamespace,omitempty"`
	// AdminServiceAccountName is the name of the serviceaccount used for the admin access.
	// Defaults to 'admin'.
	AdminServiceAccountName *string `json:"adminServiceAccountName,omitempty"`
}

type CompletedCommonConfig struct {
	ServiceAccountNamespace string
	AdminServiceAccountName string
}

// CompletedAPIServerProviderConfiguration is a helper struct. It contains the original configuration,
// enriched with defaults and processed values (e.g. a client constructed from a plaintext kubeconfig).
type CompletedAPIServerProviderConfiguration struct {
	*CompletedCommonConfig

	// ConfiguredTypes contains all types for which configuration was given in the global config.
	// The APIServerProvider will return an error for an APIServerConfiguration with a type that is not in this set.
	ConfiguredTypes sets.Set[openmcpv1alpha1.APIServerType]

	GardenerConfig *CompletedMultiGardenerConfiguration
}

// Complete transforms the APIServerProviderConfiguration into a CompletedAPIServerProviderConfiguration.
func (cfg *APIServerProviderConfiguration) Complete(ctx context.Context) (*CompletedAPIServerProviderConfiguration, error) {
	if cfg == nil {
		return nil, nil
	}
	res := &CompletedAPIServerProviderConfiguration{
		ConfiguredTypes: sets.New[openmcpv1alpha1.APIServerType](),
	}
	errs := []error{}
	var err error

	res.GardenerConfig, err = cfg.GardenerConfig.complete(ctx)
	if err == nil && res.GardenerConfig != nil {
		res.ConfiguredTypes.Insert(openmcpv1alpha1.Gardener)
	}
	errs = append(errs, err)

	res.CompletedCommonConfig, err = cfg.CommonConfig.complete()
	errs = append(errs, err)

	return res, errors.Join(errs...)
}

func Validate(cfg *APIServerProviderConfiguration) error {
	allErrs := field.ErrorList{}

	if cfg == nil {
		allErrs = append(allErrs, field.Required(field.NewPath(""), "APIServer provider configuration must not be empty"))
		return allErrs.ToAggregate()
	}

	if cfg.GardenerConfig != nil {
		allErrs = append(allErrs, validateGardenerConfig(cfg.GardenerConfig, field.NewPath("gardener"))...)
	}

	allErrs = append(allErrs, cfg.CommonConfig.validate()...)

	return allErrs.ToAggregate()
}

func (cc *CommonConfig) validate() field.ErrorList {
	// currently nothing to validate, might change in the future
	return nil
}

func (cc *CommonConfig) complete() (*CompletedCommonConfig, error) {
	res := &CompletedCommonConfig{}

	res.ServiceAccountNamespace = openmcpv1alpha1.SystemNamespace
	if cc.ServiceAccountNamespace != nil {
		res.ServiceAccountNamespace = *cc.ServiceAccountNamespace
	}
	res.AdminServiceAccountName = "admin"
	if cc.AdminServiceAccountName != nil {
		res.AdminServiceAccountName = *cc.AdminServiceAccountName
	}

	return res, nil
}
