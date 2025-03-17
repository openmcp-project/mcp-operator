package gardener

import (
	"encoding/json"

	"github.tools.sap/CoLa/controller-utils/pkg/logging"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ shootBuilder = &shootBuilderGCP{}

type shootBuilderGCP struct {
	baseShootBuilder
}

func (b *shootBuilderGCP) newControlPlaneConfig(log logging.Logger) (*runtime.RawExtension, error) {
	log.Debug("Setting shoot.Spec.Provider.ControlPlaneConfig.Zone", "value", b.controlPlaneZone)
	controlPlaneConfig := map[string]any{
		"apiVersion": "gcp.provider.extensions.gardener.cloud/v1alpha1",
		"kind":       "ControlPlaneConfig",
		"zone":       b.controlPlaneZone,
	}
	controlPlaneConfigRaw, err := json.Marshal(controlPlaneConfig)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{
		Raw: controlPlaneConfigRaw,
	}, nil
}
