package components

import (
	"fmt"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.tools.sap/CoLa/mcp-operator/api/errors"
)

type AuthorizationConverter struct{}

var _ Component = &openmcpv1alpha1.Authorization{}
var _ ComponentConverter = &AuthorizationConverter{}

// ConvertToResourceSpec implements ComponentConverter.
func (ac *AuthorizationConverter) ConvertToResourceSpec(mcp *openmcpv1alpha1.ManagedControlPlane, _ *openmcpv1alpha1.InternalConfiguration) (any, error) {
	acConfig := mcp.Spec.Authorization
	if acConfig == nil {
		return nil, fmt.Errorf("authorization configuration is missing")
	}

	res := &openmcpv1alpha1.AuthorizationSpec{
		AuthorizationConfiguration: *acConfig.DeepCopy(),
	}

	res.Default()
	if err := res.Validate("spec", "authorization"); err != nil {
		return nil, fmt.Errorf("invalid Authorization configuration: %w", err)
	}

	return res, nil
}

// InjectStatus implements ComponentConverter.
func (ac *AuthorizationConverter) InjectStatus(raw any, mcpStatus *openmcpv1alpha1.ManagedControlPlaneStatus) error {
	status, ok := raw.(*openmcpv1alpha1.ExternalAuthorizationStatus)
	if !ok {
		return openmcperrors.ErrWrongComponentStatusType
	}
	mcpStatus.Components.Authorization = status.DeepCopy()
	return nil
}

// IsConfigured implements ComponentConverter.
func (ac *AuthorizationConverter) IsConfigured(mcp *openmcpv1alpha1.ManagedControlPlane) bool {
	return mcp != nil && mcp.Spec.Authorization != nil
}
