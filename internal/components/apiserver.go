package components

import (
	"fmt"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
)

// This file contains methods for defaulting, validating, and converting APIServerConfiguration (into APIServerSpec) structs.

type APIServerConverter struct{}

var _ Component = &openmcpv1alpha1.APIServer{}
var _ ComponentConverter = &APIServerConverter{}

// ConvertToResourceSpec implements ComponentConverter.
func (*APIServerConverter) ConvertToResourceSpec(mcp *openmcpv1alpha1.ManagedControlPlane, icfg *openmcpv1alpha1.InternalConfiguration) (any, error) {
	apiServerConfig := mcp.Spec.Components.APIServer
	if apiServerConfig == nil {
		return nil, fmt.Errorf("APIServer configuration is missing")
	}

	res := &openmcpv1alpha1.APIServerSpec{
		APIServerConfiguration: *apiServerConfig.DeepCopy(),
	}
	if icfg != nil {
		if icfg.Spec.Components.APIServer != nil {
			res.Internal = icfg.Spec.Components.APIServer.DeepCopy()
		}
	}

	cc, _ := GetCommonConfig(mcp, icfg)
	if cc != nil {
		res.DesiredRegion = cc.DesiredRegion
	}

	res.Default()
	if err := res.Validate("spec", "apiserver"); err != nil {
		return nil, fmt.Errorf("invalid APIServer configuration: %w", err)
	}

	return res, nil
}

// InjectStatus implements ComponentConverter.
func (*APIServerConverter) InjectStatus(raw any, mcpStatus *openmcpv1alpha1.ManagedControlPlaneStatus) error {
	status, ok := raw.(openmcpv1alpha1.ExternalAPIServerStatus)
	if !ok {
		return openmcperrors.ErrWrongComponentStatusType
	}
	mcpStatus.Components.APIServer = status.DeepCopy()
	return nil
}

// IsConfigured implements ComponentConverter.
func (*APIServerConverter) IsConfigured(mcp *openmcpv1alpha1.ManagedControlPlane) bool {
	return mcp != nil && mcp.Spec.Components.APIServer != nil
}
