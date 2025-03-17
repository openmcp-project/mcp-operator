package constants

// General reasons
const (
	// ReasonNoConditions means that a component does not expose any conditions.
	// Most probably, the reason for this is that the resource has just been created and not been properly reconciled yet.
	ReasonNoConditions = "NoConditions"

	// ReasonDeletionWaitingForDependingComponents means that this component's deletion is waiting for other components that depend on this one to be removed first.
	ReasonDeletionWaitingForDependingComponents = "DeletionWaitingForDependingComponents"

	// ReasonWaitingForDependencies means that the component is waiting for another component that it depends on to become healthy.
	ReasonWaitingForDependencies = "WaitingForDependencies"

	// ReasonDependencyStatusInvalid means that the status of a dependency does not look like expected.
	ReasonDependencyStatusInvalid = "DependencyStatusInvalid"

	// ReasonComponentIsInDeletion can be used to signal that the current component is being deleted.
	ReasonComponentIsInDeletion = "ComponentIsInDeletion"

	// ReasonCrateClusterInteractionProblem hints at problems during the interaction with a Crate cluster.
	ReasonCrateClusterInteractionProblem = "CrateClusterInteractionProblem"

	// ReasonReconciliationError describes a generic error that occurred during reconciliation.
	ReasonReconciliationError = "ReconciliationError"

	// ReasonMissingExpectedCondition means that a condition that was expected to be present is missing.
	ReasonMissingExpectedCondition = "MissingExpectedCondition"
)

// General messages
const (
	// MessageComponentIsInDeletion can be used to signal that the current component is being deleted.
	MessageComponentIsInDeletion = "This component is being deleted."

	// MessageReconciliationError is a generic message that can be used to describe a reconciliation error.
	MessageReconciliationError = "An error occurred during reconciliation."
)

// APIServer Provider
const (
	// ReasonConfigurationProblem hints at problems with the APIServer provider configuration (configuration of this controller).
	ReasonConfigurationProblem = "ConfigurationProblem"

	// ReasonGardenClusterInteractionProblem hints at problems during the interaction with a Gardener cluster.
	ReasonGardenClusterInteractionProblem = "GardenClusterProblem"

	// ReasonShootIdentificationNotPossible means that the shoot belonging to the APIServer cannot be identified.
	ReasonShootIdentificationNotPossible = "ShootIdentificationNotPossible"

	// ReasonAPIServerAccessProvisioningNotPossible means that something went wrong creating/getting the access information for the APIServer.
	ReasonAPIServerAccessProvisioningNotPossible = "APIServerAccessProvisioningNotPossible"

	// ReasonInvalidAPIServerType means that the APIServer is not of the expected type.
	ReasonInvalidAPIServerType = "InvalidAPIServerType"

	// ReasonAuditLogProblem represents problems with setting up audit logging.
	ReasonAuditLogProblem = "AuditLogProblem"

	// ReasonWaitingForGardenerShoot implies that the Gardener shoot cluster is not yet ready.
	ReasonWaitingForGardenerShoot = "WaitingForGardenerShoot"
)

// Landscaper Connector
const (
	// ReasonLaaSCoreClusterInteractionProblem hints at problems during the interaction with a LaaS core cluster.
	ReasonLaaSCoreClusterInteractionProblem = "LaaSCoreClusterInteractionProblem"

	// ReasonWaitingForLaaS means that the component is currently waiting for the LaaS landscape to do something.
	ReasonWaitingForLaaS = "WaitingForLaaS"
)

// Cloud Orchestrator
const (
	// ReasonCOCoreClusterInteractionProblem hints at problems during the interaction with a LaaS core cluster.
	ReasonCOCoreClusterInteractionProblem = "COCoreClusterInteractionProblem"

	ReasonWaitingForCloudOrchestrator = "WaitingForCloudOrchestrator"
)

// Authentication Reconciler
const (
	// ReasonManagingOpenIDConnect indicates Creating/Updating/Deleting the OpenIDConnect resources has failed.
	ReasonManagingOpenIDConnect = "ManagingOpenIDConnectResourcesProblem"
)

// Authorization Reconciler
const (
	// ReasonManagingAuthorization indicates Creating/Updating/Deleting the authorization resources has failed.
	ReasonManagingAuthorization = "ManagingAuthorizationResourcesProblem"
)

// ManagedControlPlane Reconciler
const (
	// ReasonAllComponentsReconciledSuccessfully indicates that all components have been reconciled successfully.
	ReasonAllComponentsReconciledSuccessfully = "AllComponentsReconciledSuccessfully"
	// ReasonNotAllComponentsReconciledSuccessfully indicates that not all components have been reconciled successfully.
	ReasonNotAllComponentsReconciledSuccessfully = "NotAllComponentsReconciledSuccessfully"
)
