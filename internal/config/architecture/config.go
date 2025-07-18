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

type ArchConfig struct {
	// APIServer contains the configuration for the APIServer component v1-v2 bridge.
	APIServer BridgeConfig `json:"apiServer"`
	// Landscaper contains the configuration for the Landscaper component v1-v2 bridge.
	Landscaper BridgeConfig `json:"landscaper"`
}

// IsAllowedVersion returns whether the given version is allowed in the architecture configuration.
func (cfg BridgeConfig) IsAllowedVersion(version string) bool {
	return AllowedVersions.Has(version)
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

type BridgeConfig struct {
	// Version specifies the default version of the architecture to use.
	Version string `json:"version"`
	// AllowOverride specifies if the used version can be overridden by setting the appropriate label on the resource.
	AllowOverride bool `json:"allowOverride"`
}

func (cfg *BridgeConfig) Default() {
	if cfg == nil {
		return
	}

	if !cfg.IsAllowedVersion(cfg.Version) {
		cfg.Version = openmcpv1alpha1.ArchitectureV1 // default to v1 if not set or invalid
	}
}

func (cfg *ArchConfig) Validate() field.ErrorList {
	if cfg == nil {
		return nil
	}

	allErrs := field.ErrorList{}

	cbcs := cfg.componentBridgeConfigs()
	for ct, bridgeConfig := range cbcs {
		allErrs = append(allErrs, bridgeConfig.validate(field.NewPath(string(ct)))...)
	}

	return allErrs
}

func (cfg *BridgeConfig) validate(fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if cfg == nil {
		return allErrs
	}

	if !AllowedVersions.Has(cfg.Version) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("version"), cfg.Version, fmt.Sprintf("version must be one of [%s]", strings.Join(sets.List(AllowedVersions), ", "))))
	}

	return allErrs
}

func (cfg *ArchConfig) Default() {
	if cfg == nil {
		return
	}

	cbcs := cfg.componentBridgeConfigs()
	for _, bridgeConfig := range cbcs {
		bridgeConfig.Default()
	}
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
