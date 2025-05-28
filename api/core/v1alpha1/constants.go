package v1alpha1

const (
	// GENERAL

	// BaseDomain is the CoLa base domain.
	// Components should prefix it with their own name.
	BaseDomain = "openmcp.cloud"

	// OperationAnnotation is the general operation annotation.
	OperationAnnotation = BaseDomain + "/operation"

	// OperationAnnotationValueReconcile is the value of the operation annotation which should cause a reconcile.
	OperationAnnotationValueReconcile = "reconcile"

	// OperationAnnotationValueIgnore is the value of the operation annotation which causes the responsible controller to ignore this resource.
	OperationAnnotationValueIgnore = "ignore"

	// ManagedControlPlaneBackReferenceLabelName contains the name of the creating ManagedControlPlane resource, in case the ManagedControlPlane's status is lost.
	ManagedControlPlaneBackReferenceLabelName = BaseDomain + "/mcp-name"
	// ManagedControlPlaneBackReferenceLabelNamespace contains the namespace of the creating ManagedControlPlane resource, in case the ManagedControlPlane's status is lost.
	ManagedControlPlaneBackReferenceLabelNamespace = BaseDomain + "/mcp-namespace"
	// ManagedControlPlaneBackReferenceLabelProject contains the Project of the ManagedControlPlane resource.
	// Note that this is only set if the corresponding project can be extracted from the containing namespace's metadata.
	// This label is for user information only and has no internal usage.
	ManagedControlPlaneBackReferenceLabelProject = BaseDomain + "/mcp-project"
	// ManagedControlPlaneBackReferenceLabelWorkspace contains the Workspace of the ManagedControlPlane resource.
	// Note that this is only set if the corresponding workspace can be extracted from the containing namespace's metadata.
	// This label is for user information only and has no internal usage.
	ManagedControlPlaneBackReferenceLabelWorkspace = BaseDomain + "/mcp-workspace"

	// ManagedControlPlaneGenerationLabel contains the generation of the managedcontrolplane from which this resource was created.
	// It is used to check whether component resources are outdated.
	ManagedControlPlaneGenerationLabel = BaseDomain + "/mcp-generation"
	// InternalConfigurationGenerationLabel contains the generation of the internalconfiguration that was used for this resource, if any.
	// It is used to check whether component resources are outdated.
	InternalConfigurationGenerationLabel = BaseDomain + "/ic-generation"

	// ManagedByLabel is added to resources created by the operator.
	ManagedByLabel = BaseDomain + "/managed-by"

	CreatedByAnnotation = BaseDomain + "/created-by"

	DisplayNameAnnotation = BaseDomain + "/display-name"

	// ComponentTypeLabel is added to the component's specific resources.
	// This allows generic functions (working on client.Object) to identify the component the resource belongs to.
	ComponentTypeLabel = BaseDomain + "/component"

	DependencyFinalizerPrefix = "dependency." + BaseDomain + "/"

	// SystemNamespace is the name of the system namespace.
	// This should be used whenever a namespace is required.
	SystemNamespace = "openmcp-system"

	// ProjectWorkspaceOperatorProjectLabel is the label that the PWO attaches to a namespace if that namespace belongs to a project.
	// Technically, this should be imported from the PWO, but it is not worth the dependency.
	ProjectWorkspaceOperatorProjectLabel = "core.openmcp.cloud/project"
	// ProjectWorkspaceOperatorWorkspaceLabel is the label that the PWO attaches to a namespace if that namespace belongs to a workspace.
	// Technically, this should be imported from the PWO, but it is not worth the dependency.
	ProjectWorkspaceOperatorWorkspaceLabel = "core.openmcp.cloud/workspace"

	// MANAGEDCONTROLPLANE

	// ManagedControlPlaneDomain is the domain for the v1alpha1.ManagedControlPlane controller.
	ManagedControlPlaneDomain = "managedcontrolplane." + BaseDomain

	// ManagedControlPlaneFinalizer is the finalizer for the ManagedControlPlane resource.
	ManagedControlPlaneFinalizer = "finalizer." + ManagedControlPlaneDomain

	// ManagedControlPlaneDeletionConfirmationAnnotation is the annotation, which needs to be set true before a mcp can be deleted
	ManagedControlPlaneDeletionConfirmationAnnotation = "confirmation." + BaseDomain + "/deletion"

	// APIServer

	APIServerDomain = "apiserver." + BaseDomain

	ManagedByAPIServerLabel = APIServerDomain + "/managed"

	// Architecture Switch Labels
	ArchitectureLabelPrefix      = "architecture." + BaseDomain + "/"
	ArchitectureV2               = "v2"
	V1MCPReferenceLabelName      = "v1." + BaseDomain + "/mcp-name"
	V1MCPReferenceLabelNamespace = "v1." + BaseDomain + "/mcp-namespace"
)
