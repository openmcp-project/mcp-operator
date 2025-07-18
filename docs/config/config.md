# Configuration

The MCP Operator takes a - currently optional - configuration file via the `--config` argument.

```yaml
architecture: # the architecture configuration
  apiServer:
    version: v2
    allowOverride: true
```

The following fields can be specified:
- `architecture` _(optional)_
  - The architecture configuration has its own [documentation](../architecture-v2/bridge.md), see there for further details.
