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
  owners:
    - my-org
    - shared-actions
```

- `base_url` (required): The base URL of the GHES instance (e.g., `https://ghes.example.com`)
- `owners` (required): List of repository owners to match (exact match)

### Environment Variables

#### GHES Configuration

GHES can also be configured via environment variables (used when `.pinact.yaml` does not have `ghes` settings):

- `PINACT_GHES_BASE_URL`: GHES base URL (e.g., `https://ghes.example.com`)
- `PINACT_GHES_OWNERS`: Comma-separated list of repository owners
- `GITHUB_API_URL`: Alternative to `PINACT_GHES_BASE_URL` (commonly set in GitHub Actions on GHES)

```bash
export PINACT_GHES_BASE_URL="https://ghes.example.com"
export PINACT_GHES_OWNERS="my-org-1,my-org-2"
```

Resolution priority for base URL:
1. If `PINACT_GHES_BASE_URL` is set, it is used (and `GITHUB_API_URL` is ignored)
2. If `PINACT_GHES_BASE_URL` is not set but `GITHUB_API_URL` is set, `GITHUB_API_URL` is used

Requirements:
- If `PINACT_GHES_BASE_URL` is set, `PINACT_GHES_OWNERS` is required
- If `PINACT_GHES_OWNERS` is set, either `PINACT_GHES_BASE_URL` or `GITHUB_API_URL` is required
- If neither `PINACT_GHES_BASE_URL` nor `GITHUB_API_URL` is set, `PINACT_GHES_OWNERS` is optional (only github.com actions are processed)

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
2. For each extracted action, check if its owner matches any entry in `ghes.owners`
3. If matched, search for the action on the GHES instance
4. If not matched, search for the action on github.com (existing behavior)

### Repository Matching

- `owners`: Exact match against the repository owner
- If no owner matches, the action defaults to github.com

### Review Mode (`pinact run -review`)

When using `pinact run -review`, the review comment is created on the appropriate GitHub instance:

- If `-repo-owner` matches any entry in `ghes.owners`, the review is created on the GHES instance
- Otherwise, the review is created on github.com

## Constraints

- Only one GHES instance is supported
- Actions are NOT searched on GHES first and then fallback to github.com
  - This prevents unnecessary API requests to GHES instances
  - Users must explicitly configure which actions are hosted on GHES

## Example

### Using Configuration File

```yaml
# .pinact.yaml
ghes:
  base_url: https://ghes.example.com
  owners:
    - my-org
    - shared-actions
```

```bash
export GITHUB_TOKEN="ghp_xxxx"  # for github.com
export GHES_TOKEN="ghp_yyyy"    # for GHES
```

### Using Environment Variables Only

```bash
export GITHUB_TOKEN="ghp_xxxx"  # for github.com
export GHES_TOKEN="ghp_yyyy"    # for GHES
export PINACT_GHES_BASE_URL="https://ghes.example.com"
export PINACT_GHES_OWNERS="my-org,shared-actions"
```

### Using GITHUB_API_URL (GitHub Actions on GHES)

[When running on GitHub Actions hosted on GHES, `GITHUB_API_URL` is automatically set](https://docs.github.com/en/enterprise-server@3.19/actions/reference/workflows-and-actions/variables), so `PINACT_GHES_BASE_URL` is not required:

```bash
# GITHUB_API_URL is automatically set by GitHub Actions on GHES
# GITHUB_API_URL="https://ghes.example.com/api/v3"

export GITHUB_TOKEN="ghp_xxxx"  # for github.com
export GHES_TOKEN="ghp_yyyy"    # for GHES
export PINACT_GHES_OWNERS="$GITHUB_REPOSITORY_OWNER"
```

### Workflow

```yaml
# .github/workflows/ci.yml
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      # This repo matches "my-org" owner -> searched on ghes.example.com
      - uses: my-org/build-action@v1

      # This repo matches "shared-actions" owner -> searched on ghes.example.com
      - uses: shared-actions/common-lint@v1

      # This repo doesn't match any GHES owner -> searched on github.com
      - uses: actions/checkout@v4
```

## API Integration

For GHES instances, pinact uses the same GitHub API endpoints but with the GHES base URL:

- Base URL: `<base_url>/api/v3/` (appended automatically if not present)
- Authentication: Bearer token via GHES token environment variables

## Error Handling

- If a matching GHES token is not found, return an error with a clear message
- If the GHES API request fails, return the error without fallback to github.com
- Missing `base_url` or `owners` should be reported at startup
