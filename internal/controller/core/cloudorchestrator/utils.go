package cloudorchestrator

// copyMapEntries copies map entries from a source to a target map, both must have the the same type.
func copyMapEntries[K comparable, V any](target map[K]V, source map[K]V, keys ...K) {
	if source == nil {
		return
	}

	for _, key := range keys {
		target[key] = source[key]
	}
}
