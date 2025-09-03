package components

import (
	"fmt"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
)

type AuthenticationConverter struct{}

var _ Component = &openmcpv1alpha1.Authentication{}
var _ ComponentConverter = &AuthenticationConverter{}

// ConvertToResourceSpec implements ComponentConverter.
func (ac *AuthenticationConverter) ConvertToResourceSpec(mcp *openmcpv1alpha1.ManagedControlPlane, _ *openmcpv1alpha1.InternalConfiguration) (any, error) {
	acConfig := mcp.Spec.Authentication
	if acConfig == nil {
		acConfig = &openmcpv1alpha1.AuthenticationConfiguration{}
	}

	res := &openmcpv1alpha1.AuthenticationSpec{
		AuthenticationConfiguration: *acConfig.DeepCopy(),
	}

	res.Default()
	if err := res.Validate("spec", "authentication"); err != nil {
		return nil, fmt.Errorf("invalid Authentication configuration: %w", err)
	}

	return res, nil
}

// InjectStatus implements ComponentConverter.
func (ac *AuthenticationConverter) InjectStatus(raw any, mcpStatus *openmcpv1alpha1.ManagedControlPlaneStatus) error {
	status, ok := raw.(openmcpv1alpha1.ExternalAuthenticationStatus)
	if !ok {
		return openmcperrors.ErrWrongComponentStatusType
	}
	mcpStatus.Components.Authentication = status.DeepCopy()
	return nil
}

// IsConfigured implements ComponentConverter.
func (ac *AuthenticationConverter) IsConfigured(mcp *openmcpv1alpha1.ManagedControlPlane) bool {
	return mcp != nil && (mcp.Spec.Authentication != nil || mcp.Spec.Components.APIServer != nil)
}
