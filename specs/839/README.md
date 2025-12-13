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
  api_url: https://ghes.example.com  # /api/v3/ is appended if not present
  fallback: false  # Whether to fallback to github.com (default: false)
```

- `api_url` (optional): The API URL of the GHES instance (e.g., `https://ghes.example.com`). Can also be set via environment variables.
- `fallback` (optional): Whether to fallback to github.com when a repository is not found on GHES (default: `false`)

### Environment Variables

#### GHES Configuration

GHES can also be configured via environment variables:

- `PINACT_GHES_API_URL`: GHES API URL (e.g., `https://ghes.example.com`)
- `GITHUB_API_URL`: Alternative to `PINACT_GHES_API_URL` (commonly set in GitHub Actions on GHES)
- `PINACT_GHES_FALLBACK`: Set to `true` to enable fallback to github.com

```bash
export PINACT_GHES_API_URL="https://ghes.example.com"
export PINACT_GHES_FALLBACK="true"  # optional: enable fallback
```

Resolution priority for API URL:
1. If `PINACT_GHES_API_URL` is set, it is used (and `GITHUB_API_URL` is ignored)
2. If `PINACT_GHES_API_URL` is not set but `GITHUB_API_URL` is set and is not `https://api.github.com`, `GITHUB_API_URL` is used

#### Conditions for Enabling GHES

GHES mode is enabled when any of the following conditions are met:

1. `ghes.api_url` is configured in the configuration file
2. `PINACT_GHES_API_URL` environment variable is set
3. `GITHUB_API_URL` environment variable is set and is not `https://api.github.com`

Environment variables can also complement missing values in the configuration file:
- If `ghes.api_url` is empty in the config file, it is filled from `PINACT_GHES_API_URL` or `GITHUB_API_URL`
- `PINACT_GHES_FALLBACK=true` can enable fallback even if not set in config file

This allows using GHES without a configuration file.

#### GitHub Access Tokens

GitHub Access Tokens are specified via environment variables:

- `GITHUB_TOKEN`: Token for github.com (existing behavior)
- GHES token (checked in order, first non-empty value is used):
  1. `GHES_TOKEN`
  2. `GITHUB_TOKEN_ENTERPRISE`
  3. `GITHUB_ENTERPRISE_TOKEN`

## Behavior

1. pinact parses workflow files and extracts actions (existing behavior)
2. For each extracted action:
   - If GHES is not enabled, search on github.com (existing behavior)
   - If GHES is enabled and `fallback: false` (default), always use GHES
   - If GHES is enabled and `fallback: true`, check GHES first, then fallback to github.com if not found

### Fallback Behavior

By default, fallback is **disabled** (`fallback: false`). This aligns with GHES default behavior where GitHub Connect is disabled.

#### When `fallback: false` (default)
- All actions are searched on the GHES instance only
- No repository existence check is performed (better performance)
- If the action is not found on GHES, an error is returned

#### When `fallback: true`
- Actions are first searched on the GHES instance using the Get a Repository API
- If GHES returns 404 (not found), the action is searched on github.com
- The result is cached per repository to avoid redundant API calls
- Other errors from GHES are returned without fallback

This approach eliminates the need to maintain a list of owners and simplifies configuration. Users only need to configure the GHES API URL and optionally enable fallback.

### Review Mode (`pinact run -review`)

When using `pinact run -review`, the review comment is created on the GHES instance if GHES is enabled, otherwise on github.com. There is no fallback for PR comments - if GHES is enabled but the comment creation fails, an error is returned.

## Constraints

- Only one GHES instance is supported
- Fallback only applies to action searches, not to PR comment creation
- Fallback is disabled by default for security and consistency with GHES defaults

## Example

### Using Configuration File (without fallback)

```yaml
# .pinact.yaml
ghes:
  api_url: https://ghes.example.com
```

```bash
export GHES_TOKEN="ghp_yyyy"    # for GHES
```

### Using Configuration File (with fallback)

```yaml
# .pinact.yaml
ghes:
  api_url: https://ghes.example.com
  fallback: true
```

```bash
export GITHUB_TOKEN="ghp_xxxx"  # for github.com
export GHES_TOKEN="ghp_yyyy"    # for GHES
```

### Using Environment Variables Only

```bash
export GHES_TOKEN="ghp_yyyy"    # for GHES
export PINACT_GHES_API_URL="https://ghes.example.com"
# export PINACT_GHES_FALLBACK="true"  # optional: enable fallback
# export GITHUB_TOKEN="ghp_xxxx"  # required if fallback is enabled
```

### Using GITHUB_API_URL (GitHub Actions on GHES)

[When running on GitHub Actions hosted on GHES, `GITHUB_API_URL` is automatically set](https://docs.github.com/en/enterprise-server@3.19/actions/reference/workflows-and-actions/variables), so `PINACT_GHES_API_URL` is not required:

```bash
# GITHUB_API_URL is automatically set by GitHub Actions on GHES
# GITHUB_API_URL="https://ghes.example.com/api/v3"

export GHES_TOKEN="ghp_yyyy"    # for GHES
# export PINACT_GHES_FALLBACK="true"  # optional: enable fallback
# export GITHUB_TOKEN="ghp_xxxx"  # required if fallback is enabled
```

### Workflow

```yaml
# .github/workflows/ci.yml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      # When fallback is disabled (default): always use GHES
      # When fallback is enabled: check GHES first, fallback to github.com if not found
      - uses: my-org/build-action@v1

      # Same behavior for all actions
      - uses: actions/checkout@v4
```

## API Integration

For GHES instances, pinact uses the same GitHub API endpoints but with the GHES API URL:

- API URL: `<api_url>/api/v3/` (appended automatically if not present)
- Authentication: Bearer token via GHES token environment variables

## Error Handling

- If GHES is enabled but the GHES token is not found, return an error with a clear message
- If the GHES API request fails with non-404 error, return the error without fallback to github.com
- Missing `api_url` when GHES is enabled should be reported at startup

## Logging

- Fallback from GHES to github.com is logged at debug level to avoid noisy output (only when `fallback: true`)
- This allows users to understand the behavior without being overwhelmed by logs
