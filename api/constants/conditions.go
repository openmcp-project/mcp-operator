package constants

const (
	// ConditionMCPSuccessful is an aggregated condition showing whether all component resources could be reconciled successfully.
	ConditionMCPSuccessful = "MCPSuccessful"

	ConditionClusterRequestGranted = "ClusterRequestGranted"
	ConditionClusterReady          = "ClusterReady"
	ConditionAccessRequestGranted  = "AccessRequestGranted"
	ConditionAccessRequestDeleted  = "AccessRequestDeleted"
	ConditionClusterRequestDeleted = "ClusterRequestDeleted"
)
