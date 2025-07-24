package config

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"

	"github.com/openmcp-project/mcp-operator/internal/config/architecture"
)

var Config MCPOperatorConfig

func init() {
	Config.Default()
}

type MCPOperatorConfig struct {
	// Architecture contains the configuration regarding v1 and v2 architecture.
	Architecture architecture.ArchConfig `json:"architecture"`
}

func LoadConfig(path string) (*MCPOperatorConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}
	return LoadConfigFromBytes(data)
}

func LoadConfigFromBytes(data []byte) (*MCPOperatorConfig, error) {
	cfg := &MCPOperatorConfig{}
	err := yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}
	cfg.Default()
	return cfg, nil
}

func (cfg *MCPOperatorConfig) Default() {
	if cfg == nil {
		return
	}

	cfg.Architecture.Default()
}

func (cfg *MCPOperatorConfig) Validate() field.ErrorList {
	if cfg == nil {
		return nil
	}

	allErrs := field.ErrorList{}

	allErrs = append(allErrs, cfg.Architecture.Validate()...)

	return allErrs
}
