# Spec: GitHub Enterprise Server (GHES) Support

- [#839](https://github.com/suzuki-shunsuke/pinact/issues/839)
- [GitHub Docs: Enabling automatic access to github.com actions using GitHub Connect](https://docs.github.com/en/enterprise-server@3.19/admin/managing-github-actions-for-your-enterprise/managing-access-to-actions-from-githubcom/enabling-automatic-access-to-githubcom-actions-using-github-connect)

## Overview

Add support for pinning and updating GitHub Actions hosted on GitHub Enterprise Server (GHES). When GitHub Connect is enabled, actions from github.com are also supported alongside GHES-hosted actions.

## Configuration

### Configuration File

GHES settings are defined in the configuration file (`.pinact.yaml`):

```yaml
ghes:
  base_url: https://ghes.example.com  # /api/v3/ is appended if not present
  actions:
    - foo/.*
    - suzuki-shunsuke/enterprise-action
```

- `base_url`: The base URL of the GHES instance (e.g., `https://ghes.example.com`)
- `actions`: List of regular expression patterns to match action names (format: `owner/repo`)

The configuration file is required when using GHES.

### Environment Variables

GitHub Access Tokens are specified via environment variables:

- `GITHUB_TOKEN`: Token for github.com (existing behavior)
- GHES token (checked in order, first non-empty value is used):
  1. `GHES_TOKEN`
  2. `GITHUB_TOKEN_ENTERPRISE`
  3. `GITHUB_ENTERPRISE_TOKEN`

## Behavior

1. pinact parses workflow files and extracts actions (existing behavior)
2. For each extracted action, check if it matches any pattern in `ghes.actions`
3. If matched, search for the action on the GHES instance
4. If not matched, search for the action on github.com (existing behavior)

### Action Matching

- Actions are matched using regular expressions against the `owner/repo` portion
- If no GHES pattern matches, the action defaults to github.com

## Constraints

- Configuration file is required when using GHES
- Only one GHES instance is supported
- Actions are NOT searched on GHES first and then fallback to github.com
  - This prevents unnecessary API requests to GHES instances
  - Users must explicitly configure which actions are hosted on GHES

## Example

### Configuration

```yaml
# .pinact.yaml
ghes:
  base_url: https://ghes.example.com
  actions:
    - my-org/.*
    - shared-actions/common-.*
```

### Environment

```bash
export GITHUB_TOKEN="ghp_xxxx"  # for github.com
export GHES_TOKEN="ghp_yyyy"    # for GHES
```

### Workflow

```yaml
# .github/workflows/ci.yml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      # This action matches "my-org/.*" pattern -> searched on ghes.example.com
      - uses: my-org/build-action@v1

      # This action doesn't match any GHES pattern -> searched on github.com
      - uses: actions/checkout@v4
```

## API Integration

For GHES instances, pinact uses the same GitHub API endpoints but with the GHES base URL:

- Base URL: `<base_url>/api/v3/` (appended automatically if not present)
- Authentication: Bearer token via GHES token environment variables

## Error Handling

- If a matching GHES token is not found, return an error with a clear message
- If the GHES API request fails, return the error without fallback to github.com
- Invalid regex patterns in configuration should be reported at startup
