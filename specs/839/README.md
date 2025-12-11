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
  - host: ghes.example.com
    actions:
      - foo/.*
      - suzuki-shunsuke/enterprise-action
```

- `host`: The hostname of the GHES instance
- `actions`: List of regular expression patterns to match action names (format: `owner/repo`)

The configuration file is required when using GHES.

### Environment Variables

GitHub Access Tokens are specified via environment variables:

- `GITHUB_TOKEN`: Token for github.com (existing behavior)
- `GITHUB_TOKEN_<host>`: Token for GHES instance
  - The host name is lowercase and dots/hyphens are replaced with underscores
  - Example: For `ghes.example.com`, the environment variable is `GITHUB_TOKEN_ghes_example_com`

This naming convention is inspired by:

- [Terraform CLI Environment Variable Credentials](https://developer.hashicorp.com/terraform/cli/config/config-file#environment-variable-credentials)
- [tflint discussion on GHES support](https://github.com/terraform-linters/tflint/issues/2005#issuecomment-2002166525)

## Behavior

1. pinact parses workflow files and extracts actions (existing behavior)
2. For each extracted action, check if it matches any pattern in `ghes[].actions`
3. If matched, search for the action on the corresponding GHES instance
4. If not matched, search for the action on github.com (existing behavior)

### Action Matching

- Actions are matched using regular expressions against the `owner/repo` portion
- The first matching GHES configuration is used
- If no GHES pattern matches, the action defaults to github.com

## Constraints

- Configuration file is required when using GHES
- Actions are NOT searched on GHES first and then fallback to github.com
  - This prevents unnecessary API requests to GHES instances
  - Users must explicitly configure which actions are hosted on GHES

## Example

### Configuration

```yaml
# .pinact.yaml
ghes:
  - host: ghes.example.com
    actions:
      - my-org/.*
      - shared-actions/common-.*
```

### Environment

```bash
export GITHUB_TOKEN="ghp_xxxx"  # for github.com
export GITHUB_TOKEN_ghes_example_com="ghp_yyyy"  # for ghes.example.com
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

- Base URL: `https://<host>/api/v3`
- Authentication: Bearer token via `GITHUB_TOKEN_<host>` environment variable

## Error Handling

- If a matching GHES token is not found, return an error with a clear message
- If the GHES API request fails, return the error without fallback to github.com
- Invalid regex patterns in configuration should be reported at startup
