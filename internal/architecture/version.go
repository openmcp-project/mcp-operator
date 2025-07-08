package architecture

import (
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	"github.com/openmcp-project/mcp-operator/internal/architecture/config"
	"github.com/openmcp-project/mcp-operator/internal/components"
)

var Config config.ArchConfig

func init() {
	Config = config.ArchConfig{}
	(&Config).Default()
}

// DecideVersion determines the architecture version for a given component.
// This basically just checks the component's labels for the architecture version label.
// If the label is missing, the configured default version for the component type is returned.
// If the component is nil, 'v1' is returned.
func DecideVersion(comp components.Component) string {
	if comp == nil {
		return openmcpv1alpha1.ArchitectureV1
	}

	bridgeConfig := Config.GetBridgeConfigForComponent(comp.Type())
	version := bridgeConfig.Version

	labelVersion, ok := comp.GetLabels()[openmcpv1alpha1.ArchitectureVersionLabel]
	if ok && bridgeConfig.IsAllowedVersion(labelVersion) {
		version = labelVersion
	}

	return version
}
