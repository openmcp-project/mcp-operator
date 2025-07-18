# v2 Architecture Bridge

In order to migrate an existing MCP landscape to the new v2 architecture step by step, additional logic was added to the MCP operator, which allows it to switch between the old and the new logic for each component. The 'new logic' is usually just some bridge logic that transforms the v1 api type into the v2 api type and transforms the v2 api type's status back into the v1 format.

The bridge is currently implemented for the following components:
- `APIServer`

## Architecture Configuration

To configure for which components the bridge is enabled, set the architecture config in the [general configuration](../config/config.md) file:
```yaml
immutability:
  policyName: mcp-architecture-immutability
  disabled: false
apiServer:
  version: v1
  allowOverride: false
# more components are to follow
```

The component configuration should look similar, if not identical, for each component:
- `version` describes the architecture version that is used for this component by default.
  - Valid values are `v1` (meaning old logic) and `v2` (using the v2 bridge).
  - Defaults to `v1` if not specified for a component.
- `allowOverride` specifies whether the version specified in `version` should be able to be overridden by a corresponding label on the `ManagedControlPlane` resource.
  - If this is `true` for a specific component and an MCP resource has a label `<lowercase_component_name>.architecture.openmcp.cloud/version`, the label's value will be used instead of the version configured in the architecture configuration.
    - For example, the label for the `APIServer` component is named `apiserver.architecture.openmcp.cloud/version`.
    - If `allowOverride` is `false`, setting such a label on the MCP resource causes an error during reconciliation.
    - If the label's value is not a valid version, an error will occur during reconciliation.
  - Defaults to `false` if not specified for a component.

## Architecture Version Labels and Immutability

The architecture that is used for a specific component of a specific MCP must not be changed after it has been initially decided. The reason for this is simple: If the version was changed from `v1` to `v2` after the component has already been deployed, the `v2` bridge logic would not detect the resources that were already deployed by the `v1` logic and re-deploy it 'the v2 way', leading to duplicated resources and potential conflicts. The same is true vice-versa.

To ensure that the architecture version does not change, we use a combination of labels and [ValidatingAdmissionPolicies](https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/):
- The 'ground truth' of which version is being used is stored in a label on each component resource.
  - This is used by the components' controllers to decide which logic they use for reconciliation.
  - The label's key is `architecture.openmcp.cloud/version`.
  - As a kind of migration, component resources that don't have the label are treated as having it set to `v1`.
- The value of the label is never allowed to change.
  - If the label is missing, it is allowed to be added with `v1` as value.
- Newly created or updated component resources must have the label set.

If `immutability.disabled` in the architecture configuration is not set to `true` (it is `false` by default), the MCP operator will deploy a `ValidatingAdmissionPolicy` and `ValidatingAdmissionPolicyBinding` on startup to ensure the architecture version immutability. `immutability.policyName` specifies the name for both resources and defaults to `mcp-architecture-immutability` if not specified.

Both resources are removed during startup if `immutability.disabled` is set to `true`.
