# Default audit logging for v1 Gardener MCPs

The API-server provider configuration loaded with `--apiserver-config` supports
an optional, operator-controlled `defaultAuditLog` block. It applies to the v1
Gardener handler (`Gardener` and `GardenerDedicated`), not the v2 ClusterProvider /
service-provider auditlog path.

Add the block alongside the existing `gardener` provider configuration, **not**
to the MCP manifest or the operator's general `--config` file:

```yaml
# Existing gardener provider configuration remains unchanged.
defaultAuditLog:
  type: standard
  tenantID: <platform-audit-tenant-id>
  serviceURL: https://audit.example.com
  secretRef:
    name: platform-audit-credentials
    namespace: operator-secrets
  policyRef:
    name: platform-audit-policy
    namespace: operator-policies
```

All fields are required when the block is present. References must include a
valid namespace and resource name; the service URL must be absolute HTTPS without
embedded credentials or a fragment. Gardener must permit the configured endpoint.
Removing the entire block disables the fallback. Existing configurations without
this block retain their previous behavior.

## Precedence and removal

1. An MCP's `spec.components.apiServer.gardener.auditLog`, propagated to
   `APIServer.spec.gardener.auditLog`, takes precedence in its entirety.
2. If that block is absent, the operator uses `defaultAuditLog`.
3. If both are absent, the operator removes the generated audit configuration and
   its per-shoot policy/credential resources, as before.

An invalid explicit configuration, or unavailable explicit source resources,
produces a reconciliation error; it never silently switches the customer's
endpoint/tenant to the default. Removing an explicit configuration while the
fallback is enabled **switches to platform audit logging**, rather than disabling
logging. This feature does not add a customer opt-out.

The fallback is resolved internally. The operator does not insert it into the
MCP or generated APIServer spec, expose central source references there, or modify
other MCP/API-server configuration. Shoot deletion cleans up generated per-shoot
resources without deleting central sources or reading/reapplying the fallback.

## Source resources and permissions

The Secret and ConfigMap must exist on the **crate cluster used by the v1 handler**.
Only the operator configuration can specify their namespaces. Customer MCP
references remain local to the customer's namespace; this change does not allow
customers to choose arbitrary cross-namespace references.

- Provision credentials through the platform's approved secret-management
  mechanism. The Gardener auditlog extension expects the `user` and `password`
  keys. Do not put credentials in the provider configuration.
- Store the approved Kubernetes audit policy under the ConfigMap's `data.policy`.
- Ensure the operator's crate client can read the central resources and that its
  cache/watch scope includes those namespaces when applicable. Source changes
  take effect on the next API-server reconciliation; no new cross-namespace
  watch is introduced by this feature.
- Do **not** grant customer principals access to these namespaces. Restrict write
  access to the operator configuration and source resources to platform admins.

The controller copies the central resources into the configured **Gardener
project namespace** as `<shoot>--auditlog-credentials` and `<shoot>--auditlog-policy`
(using its existing name-prefix logic). The shoot extension references the
per-shoot Secret through alias `auditlog-credentials`. Credentials are not copied
into the customer's MCP namespace or exposed through the MCP spec. They remain
accessible to principals authorized to read Secrets in the Gardener project;
central source placement is not a replacement for Gardener RBAC.

## Activation

Deploying a build with this feature alone does not enable it. Provision and
permission the central resources first, then add `defaultAuditLog` to the provider
configuration and restart/reload the operator through the normal deployment
process. Start with a test MCP, verify its generated references and a delivered
audit event, and confirm the configuration survives subsequent reconciliation
before expanding.

For a shared platform tenant, review audit-data access, retention, residency,
customer isolation and endpoint capacity before activation. Disabling the default
will remove default-managed audit logging on subsequent reconciliations for MCPs
without an explicit configuration; this is not a preserve-existing switch.
