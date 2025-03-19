package v1alpha1

// NamespacedObjectReference is a reference to a namespaced k8s object.
type NamespacedObjectReference struct {
	// Name is the object's name.
	Name string `json:"name"`
	// Namespace is the object's namespace.
	Namespace string `json:"namespace"`
}

// SecretReference is a reference to a specific key inside a secret.
type SecretReference struct {
	NamespacedObjectReference `json:",inline"`
	// Key is the key inside the secret.
	Key string `json:"key"`
}

// LocalSecretReference is a reference to a specific key inside a secret in the same namespace
// as the object referencing it.
type LocalSecretReference struct {
	// Name is the secret name.
	Name string `json:"name"`
	// Key is the key inside the secret.
	Key string `json:"key"`
}

// SingleOrMultiStringValue is a type that can hold either a single string value or a list of string values.
type SingleOrMultiStringValue struct {
	// Value is a single string value.
	Value string `json:"value,omitempty"`
	// Values is a list of string values.
	Values []string `json:"values,omitempty"`
}
