package config

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

// LoadConfig reads the configuration file from a given path and parses it into an AuthorizationConfig object.
func LoadConfig(path string) (*AuthorizationConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}
	cfg := &AuthorizationConfig{}
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %w", err)
	}
	return cfg, nil
}
