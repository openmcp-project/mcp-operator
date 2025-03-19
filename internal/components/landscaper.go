package components

import (
	"fmt"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
)

// This file contains methods for defaulting, validating, and converting LandscaperConfiguration (into LandscaperSpec) structs.

type LandscaperConverter struct{}

var _ Component = &openmcpv1alpha1.Landscaper{}
var _ ComponentConverter = &LandscaperConverter{}

// ConvertToResourceSpec implements ComponentConverter.
func (*LandscaperConverter) ConvertToResourceSpec(mcp *openmcpv1alpha1.ManagedControlPlane, _ *openmcpv1alpha1.InternalConfiguration) (any, error) {
	lcConfig := mcp.Spec.Components.Landscaper
	if lcConfig == nil {
		return nil, fmt.Errorf("landscaper configuration is missing")
	}

	res := &openmcpv1alpha1.LandscaperSpec{
		LandscaperConfiguration: *lcConfig.DeepCopy(),
	}

	res.Default()
	if err := res.Validate("spec", "landscaper"); err != nil {
		return nil, fmt.Errorf("invalid Landscaper configuration: %w", err)
	}

	return res, nil
}

// InjectStatus implements ComponentConverter.
func (*LandscaperConverter) InjectStatus(raw any, mcpStatus *openmcpv1alpha1.ManagedControlPlaneStatus) error {
	status, ok := raw.(*openmcpv1alpha1.ExternalLandscaperStatus)
	if !ok {
		return openmcperrors.ErrWrongComponentStatusType
	}
	mcpStatus.Components.Landscaper = status.DeepCopy()
	return nil
}

// IsConfigured implements ComponentConverter.
func (*LandscaperConverter) IsConfigured(mcp *openmcpv1alpha1.ManagedControlPlane) bool {
	return mcp != nil && mcp.Spec.Components.Landscaper != nil
}
