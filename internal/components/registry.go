package components

import (
	openmcpv1alpha1 "github.tools.sap/CoLa/mcp-operator/api/core/v1alpha1"
)

// ComponentRegistry can be used to fetch the known components.
type ComponentRegistry[T ManagedComponent] interface {
	// GetKnownComponents returns a mapping from all registered component types to their respective in-cluster resources.
	// The resources are provided via the ManagedComponent interface to allow a registry to return more than just the in-cluster resource.
	// Any modification of the returned map or any of its contents must not influence the return value of future calls (= this function has to return a deep copy of its internal representation).
	GetKnownComponents() map[openmcpv1alpha1.ComponentType]T

	// GetComponent is a shorthand for 'GetKnownComponents()[comp]'.
	// It returns nil if no component is registered for the given type.
	// Any modification of the returned object must not influence the return value of future calls (= this function has to return a deep copy of its internal representation).
	GetComponent(openmcpv1alpha1.ComponentType) T

	// Register registers a new component.
	// The given function is supposed to return a 'fresh' ManagedComponent, so that each call to 'GetComponent' or 'GetKnownComponents' returns a new object.
	// The type is used as key, calling this function multiple times with the same type argument will cause the last call to overwrite anything registered with the previous ones.
	// Calling Register with a nil function is expected to unregister the given component type.
	Register(openmcpv1alpha1.ComponentType, func() T)

	// Has returns true if the given component type is registered in this registry.
	Has(openmcpv1alpha1.ComponentType) bool
}

// ManagedComponent is a helper interface that wraps the ability to return the in-cluster representation of a component and install the component's scheme.
type ManagedComponent interface {
	// Resource returns the in-cluster resource for the given component.
	// This is expected to return a pointer to the resource object, so it can be modified.
	Resource() Component
}
