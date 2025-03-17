package components

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

// Component is a helper interface which must be implemented by all component-specific in-cluster resources.
// It inherits client.Object, so it can be used instead of client.Object for the component-specific resources.
type Component interface {
	client.Object

	// Type returns the type of this component.
	Type() openmcpv1alpha1.ComponentType

	// GetSpec returns a pointer to the spec of the component.
	GetSpec() any

	// SetSpec is used by the ManagedControlPlane controller to pass the configuration from the ManagedControlPlane's spec into the spec of the component resource.
	// Returns ErrWrongComponentConfigType if cfg cannot be cast to the correct type to put into the component resource's spec.
	// This function expects cfg to be a pointer to the component's spec.
	SetSpec(cfg any) error

	// GetCommonStatus returns the part of the component's status that all components have in common.
	GetCommonStatus() openmcpv1alpha1.CommonComponentStatus

	// SetCommonStatus is used to update the common status of the component.
	SetCommonStatus(status openmcpv1alpha1.CommonComponentStatus)

	// GetExternalStatus returns a pointer to the component's external status.
	// This is used by the ManagedControlPlane controller to fetch the component's status and put it into the ManagedControlPlane's status.
	// The returned value will be fed into the corresponding ComponentConverter's InjectStatus method.
	GetExternalStatus() any

	// GetRequiredConditions returns a set of types of conditions that are expected to be present in the component's status.
	// This set can be static, but it can also depend on the component's spec.
	// All condition types that are returned by this method but don't have a matching condition in the component's status will be added with status 'Unknown' (leading to an unhealthy MCP).
	// Additional conditions in the component's status that are not in this list will still be propagated to the MCP's status (think of this as a set of minimal required conditions).
	GetRequiredConditions() sets.Set[string]
}

// GetCommonConfig takes the same arguments as the ConvertToResourceSpec function and returns the common configuration for ManagedControlPlane and InternalConfiguration.
// Both return values may be nil if no common configuration exists.
func GetCommonConfig(mcp *openmcpv1alpha1.ManagedControlPlane, icfg *openmcpv1alpha1.InternalConfiguration) (*openmcpv1alpha1.CommonConfig, *openmcpv1alpha1.InternalCommonConfig) {
	var cc *openmcpv1alpha1.CommonConfig
	if mcp != nil {
		cc = mcp.Spec.CommonConfig
	}
	var icc *openmcpv1alpha1.InternalCommonConfig
	if icfg != nil {
		icc = icfg.Spec.InternalCommonConfig
	}
	return cc, icc
}
