# workspace

Workspace commands: list accessible workspaces for the current user.

## workspace list

List all workspaces that the current user has access to.

### Usage

```
fornax-cli workspace list [flags]
```

### Description

Fetches and displays all workspaces accessible to the current user.
Requires prior SSO login (via `fornax-cli auth login`) or valid AK/SK credentials.

### Output Fields

| Field | Description |
|-------|-------------|
| `space_id` | Workspace ID (used with `--workspace-id` or `config set workspace-id`) |
| `name` | Workspace display name |
| `description` | Workspace description |

### Examples

```bash
# Pretty table output (default)
fornax-cli workspace list

# JSON output (for scripts/agents)
fornax-cli workspace list --format json

# Compact JSON output
fornax-cli workspace list --format raw

# List workspaces in a specific region
fornax-cli workspace list --custom-region SG
```

### Typical Workflow

```bash
# 1. Login via SSO
fornax-cli auth login

# 2. List available workspaces
fornax-cli workspace list

# 3. Set the desired workspace
fornax-cli config set workspace-id <SPACE_ID>
```

### Notes

- This command does NOT modify any configuration. To set the workspace-id after listing, use `fornax-cli config set workspace-id <ID>`.
- For interactive selection (pick from a list and auto-save), use `fornax-cli config select-workspace` instead.
- The `space_id` returned can be used with `--workspace-id` flag, `FORNAX_WORKSPACE_ID` env var, or `fornax-cli config set workspace-id`.
