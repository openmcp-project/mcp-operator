package architecture

import (
	"fmt"
	"maps"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	"github.com/openmcp-project/mcp-operator/internal/components"
)

var AllowedVersions = sets.New(openmcpv1alpha1.ArchitectureV1, openmcpv1alpha1.ArchitectureV2)

////////////////
// ArchConfig //
////////////////

type ArchConfig struct {
	// Immutability contains the configuration for the immutability check.
	Immutability ImmutabilityConfig `json:"immutability"`
	// APIServer contains the configuration for the APIServer component v1-v2 bridge.
	APIServer BridgeConfig `json:"apiServer"`
	// Landscaper contains the configuration for the Landscaper component v1-v2 bridge.
	Landscaper BridgeConfig `json:"landscaper"`
}

func (cfg *ArchConfig) Validate() field.ErrorList {
	if cfg == nil {
		return nil
	}

	allErrs := field.ErrorList{}

	allErrs = append(allErrs, cfg.Immutability.Validate()...)

	cbcs := cfg.componentBridgeConfigs()
	for ct, bridgeConfig := range cbcs {
		allErrs = append(allErrs, bridgeConfig.Validate(field.NewPath(string(ct)))...)
	}

	return allErrs
}

func (cfg *ArchConfig) Default() {
	if cfg == nil {
		return
	}

	cfg.Immutability.Default()

	cbcs := cfg.componentBridgeConfigs()
	for _, bridgeConfig := range cbcs {
		bridgeConfig.Default()
	}
}

// GetBridgeConfigForComponent returns the bridge configuration for the given component type.
// If the component type is not recognized, it returns a default BridgeConfig.
func (cfg *ArchConfig) GetBridgeConfigForComponent(compType openmcpv1alpha1.ComponentType) BridgeConfig {
	if cfg != nil {
		cbcs := cfg.componentBridgeConfigs()
		if bridgeConfig, ok := cbcs[compType]; ok {
			return *bridgeConfig
		}
	}
	res := BridgeConfig{}
	res.Default()
	return res
}

// componentBridgeConfigs returns a mapping from component types to their respective bridge configurations.
// Note that pointers to the BridgeConfigs are returned, but you should only modify them if you know what you're doing.
func (cfg *ArchConfig) componentBridgeConfigs() map[openmcpv1alpha1.ComponentType]*BridgeConfig {
	res := make(map[openmcpv1alpha1.ComponentType]*BridgeConfig)

	if cfg == nil {
		return res
	}

	v := reflect.ValueOf(cfg).Elem()
	for i := range v.NumField() {
		field := v.Type().Field(i)
		compIter := maps.Keys(components.Registry.GetKnownComponents())
		for ct := range compIter {
			if field.Name == string(ct) {
				if bridgeConfig, ok := v.Field(i).Addr().Interface().(*BridgeConfig); ok {
					res[ct] = bridgeConfig
				}
				break
			}
		}
	}

	return res
}

//////////////////
// BridgeConfig //
//////////////////

type BridgeConfig struct {
	// Version specifies the default version of the architecture to use.
	Version string `json:"version"`
	// AllowOverride specifies if the used version can be overridden by setting the appropriate label on the resource.
	AllowOverride bool `json:"allowOverride"`
}

// IsAllowedVersion returns whether the given version is allowed in the architecture configuration.
func (cfg BridgeConfig) IsAllowedVersion(version string) bool {
	return AllowedVersions.Has(version)
}

func (cfg *BridgeConfig) Default() {
	if cfg == nil {
		return
	}

	if !cfg.IsAllowedVersion(cfg.Version) {
		cfg.Version = openmcpv1alpha1.ArchitectureV1 // default to v1 if not set or invalid
	}
}

func (cfg *BridgeConfig) Validate(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if cfg == nil {
		return allErrs
	}

	if !AllowedVersions.Has(cfg.Version) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("version"), cfg.Version, fmt.Sprintf("version must be one of [%s]", strings.Join(sets.List(AllowedVersions), ", "))))
	}

	return allErrs
}

////////////////////////
// ImmutabilityConfig //
////////////////////////

type ImmutabilityConfig struct {
	// PolicyName is the name of the ValidatingAdmissionPolicy and the corresponding ValidatingAdmissionPolicyBinding
	// that is created to prevent switching between architecture versions.
	// Defaults to 'mcp-architecture-immutability' if not specified.
	PolicyName string `json:"policyName"`
	// Disabled disables the immutability check if set to 'true'.
	// This will cause the MCP operator to delete any previously (for the purpose of preventing architecture version changes)
	// created ValidatingAdmissionPolicy and ValidatingAdmissionPolicyBinding.
	Disabled bool `json:"disabled"`
}

func (cfg *ImmutabilityConfig) Default() {
	if cfg == nil {
		return
	}

	if cfg.PolicyName == "" {
		cfg.PolicyName = "mcp-architecture-immutability"
	}
}

func (cfg *ImmutabilityConfig) Validate() field.ErrorList {
	if cfg == nil {
		return nil
	}

	allErrs := field.ErrorList{}

	if cfg.PolicyName == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("policyName"), "policy name must not be empty"))
	}

	return allErrs
}
