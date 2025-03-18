package components

import (
	openmcpv1alpha1 "github.com/openmcp-project/mcp-operator/api/core/v1alpha1"
)

// ComponentConverter contains functions which require knowledge about how the component is configured in the ManagedControlPlane.
type ComponentConverter interface {
	// ConvertToResourceSpec converts the ManagedControlPlane spec and a potential InternalConfiguration into the component resource's spec.
	// The result of this function will be fed into the SetSpec function by the ManagedControlPlane controller.
	ConvertToResourceSpec(mcp *openmcpv1alpha1.ManagedControlPlane, ic *openmcpv1alpha1.InternalConfiguration) (any, error)

	// IsConfigured returns true if the given ManagedControlPlane contains configuration for this component.
	IsConfigured(mcp *openmcpv1alpha1.ManagedControlPlane) bool

	// InjectStatus injects the external status of this component into the ManagedControlPlane's status.
	// It must only modify the fields that are specific to this component, excluding conditions and observed generations.
	// Should throw an ErrWrongComponentStatusType error if the given object cannot be converted into the type specified in the ManagedControlPlane's status.
	InjectStatus(comp any, mcpStatus *openmcpv1alpha1.ManagedControlPlaneStatus) error
}
