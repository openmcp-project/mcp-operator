package components

import (
	"fmt"

	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
	openmcperrors "github.com/openmcp-project/mcp-operator/api/errors"
)

// This file contains methods for defaulting, validating, and converting CloudOrchestratorConfiguration (into CloudOrchestratorSpec) structs.

var _ Component = &openmcpv1alpha1.CloudOrchestrator{}
var _ ComponentConverter = &CloudOrchestratorConverter{}

// +kubebuilder:object:generate=false
type CloudOrchestratorConverter struct{}

// ConvertToResourceSpec implements ComponentConverter.
func (*CloudOrchestratorConverter) ConvertToResourceSpec(mcp *openmcpv1alpha1.ManagedControlPlane, _ *openmcpv1alpha1.InternalConfiguration) (any, error) {
	coCfg := mcp.Spec.Components.CloudOrchestratorConfiguration
	res := &openmcpv1alpha1.CloudOrchestratorSpec{
		CloudOrchestratorConfiguration: *coCfg.DeepCopy(),
	}

	res.Default()
	if err := res.Validate("spec", "cloudorchestrator"); err != nil {
		return nil, fmt.Errorf("invalid CloudOrchestrator configuration: %w", err)
	}

	return res, nil
}

// InjectStatus implements ComponentConverter.
func (*CloudOrchestratorConverter) InjectStatus(raw any, mcpStatus *openmcpv1alpha1.ManagedControlPlaneStatus) error {
	status, ok := raw.(*openmcpv1alpha1.ExternalCloudOrchestratorStatus)
	if !ok {
		return openmcperrors.ErrWrongComponentStatusType
	}
	mcpStatus.Components.CloudOrchestrator = status.DeepCopy()
	return nil
}

// IsConfigured implements ComponentConverter.
func (*CloudOrchestratorConverter) IsConfigured(mcp *openmcpv1alpha1.ManagedControlPlane) bool {
	return mcp != nil && (mcp.Spec.Components.Crossplane != nil || mcp.Spec.Components.BTPServiceOperator != nil || mcp.Spec.Components.ExternalSecretsOperator != nil || mcp.Spec.Components.Kyverno != nil || mcp.Spec.Components.Flux != nil)
}
