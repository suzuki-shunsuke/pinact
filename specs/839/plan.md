# Implementation Plan: GitHub Enterprise Server (GHES) Support

## Overview

This document outlines the implementation plan for adding GHES support to pinact.

## Key Design Decisions

### 1. Single GHES Instance

pinact supports one GHES instance in addition to github.com:
- One client for github.com (default)
- One client for the configured GHES instance

### 2. Repository Routing

Repositories are routed to GHES or github.com based on:
- `owners`: Exact match against the repository owner
- `repos`: Regular expression match against `owner/repo`

These are evaluated with OR logic - at least one of them must be configured.

### 3. Token Management

Tokens are retrieved from environment variables:
- `GITHUB_TOKEN` for github.com
- GHES token (checked in order):
  1. `GHES_TOKEN`
  2. `GITHUB_TOKEN_ENTERPRISE`
  3. `GITHUB_ENTERPRISE_TOKEN`

## Implementation Steps

### Step 1: Update Configuration Schema

**File:** `pkg/config/config.go`

Change GHES from array to single object:

```go
type Config struct {
    Version       int             `json:"version,omitempty" jsonschema:"enum=2,enum=3"`
    Files         []*File         `json:"files,omitempty"`
    IgnoreActions []*IgnoreAction `json:"ignore_actions,omitempty" yaml:"ignore_actions"`
    GHES          *GHES           `json:"ghes,omitempty" yaml:"ghes"`
}

type GHES struct {
    BaseURL      string   `json:"base_url" yaml:"base_url"`
    Owners       []string `json:"owners,omitempty"`
    Repos        []string `json:"repos,omitempty"`
    repoPatterns []*regexp.Regexp
}
```

Update GHES methods:

```go
func (g *GHES) Init() error {
    // Validate base_url
    // Validate that at least one of owners or repos is configured
    // Compile repo patterns as regular expressions
}

func (g *GHES) Match(owner, repoFullName string) bool {
    // Check if owner matches any entry in Owners (exact match)
    // OR if repoFullName matches any pattern in Repos (regex match)
}
```

### Step 2: Update Token Retrieval

**File:** `pkg/github/github.go`

```go
func GetGHESToken() string {
    // Check in order: GHES_TOKEN, GITHUB_TOKEN_ENTERPRISE, GITHUB_ENTERPRISE_TOKEN
    for _, envName := range []string{"GHES_TOKEN", "GITHUB_TOKEN_ENTERPRISE", "GITHUB_ENTERPRISE_TOKEN"} {
        if token := os.Getenv(envName); token != "" {
            return token
        }
    }
    return ""
}
```

### Step 3: Simplify ClientRegistry

**File:** `pkg/github/registry.go`

```go
type ClientRegistry struct {
    defaultClient *Client
    ghesClient    *Client
    ghesConfig    *config.GHES
}

func NewClientRegistry(ctx context.Context, defaultClient *Client, ghes *config.GHES) (*ClientRegistry, error) {
    // Create GHES client if config exists
}

func (r *ClientRegistry) ResolveHost(repoName string) bool {
    // Returns true if repo should use GHES
}
```

### Step 4: Simplify Controller

**File:** `pkg/controller/run/controller.go`

```go
type Controller struct {
    // Existing fields...
    ghesRepoService RepositoriesService
    ghesGitService  *GitServiceImpl
    clientRegistry  ClientRegistry
}

func (c *Controller) getRepositoriesService(repoName string) RepositoriesService {
    if c.clientRegistry != nil && c.clientRegistry.ResolveHost(repoName) {
        return c.ghesRepoService
    }
    return c.repositoriesService
}
```

### Step 5: Update parse_line.go

**File:** `pkg/controller/run/parse_line.go`

Remove `actionName` parameter from `getLatestVersion` and related methods. Instead, get the appropriate service once at the beginning of each method using the action's repository name.

```go
func (c *Controller) parseNoTagLine(ctx context.Context, logger *slog.Logger, action *Action) (string, error) {
    repoService := c.getRepositoriesService(action.Name)
    // Use repoService for all API calls in this method
}
```

### Step 6: Update github.go

**File:** `pkg/controller/run/github.go`

Remove `actionName` parameter from:
- `getLatestVersion`
- `getLatestVersionFromReleases`
- `getLatestVersionFromTags`
- `checkTagCooldown`

These methods will receive the appropriate service as a parameter or through the controller.

### Step 7: Update CLI Integration

**File:** `pkg/cli/run/command.go`

```go
// Set up GHES support if configured
if cfg.GHES != nil {
    registry, err := github.NewClientRegistry(ctx, gh, cfg.GHES)
    if err != nil {
        return fmt.Errorf("create GitHub client registry: %w", err)
    }
    ctrl.SetClientRegistry(registry)
    ctrl.SetGHESServices(/* single service */)
}
```

### Step 8: Update JSON Schema

**File:** `json-schema/pinact.json`

```json
"ghes": {
  "type": "object",
  "properties": {
    "base_url": {
      "type": "string",
      "description": "Base URL of the GHES instance"
    },
    "owners": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Repository owners to match (exact match)"
    },
    "repos": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Regular expression patterns to match repository names"
    }
  },
  "required": ["base_url"]
}
```

Note: At least one of `owners` or `repos` must be configured (validated at runtime).

## File Change Summary

| File | Change Type | Description |
|------|-------------|-------------|
| `pkg/config/config.go` | Modify | Add owners field, update Match to check both owners and repos |
| `pkg/github/github.go` | Modify | Simplify GetGHESToken to check multiple env vars |
| `pkg/github/registry.go` | Modify | Simplify to single GHES client |
| `pkg/controller/run/controller.go` | Modify | Simplify to single GHES service |
| `pkg/controller/run/github.go` | Modify | Remove actionName parameter from version methods |
| `pkg/controller/run/parse_line.go` | Modify | Get service once per method |
| `pkg/cli/run/command.go` | Modify | Update initialization for single GHES |
| `json-schema/pinact.json` | Modify | Add owners field, make repos optional |

## Error Handling

1. **Missing GHES token**: Return clear error message listing expected environment variables
2. **Invalid regex pattern**: Report error during config initialization with pattern details
3. **GHES API failure**: Return error without fallback to github.com
4. **Invalid base_url**: Validate URL format during config initialization
5. **Missing owners and repos**: Return error if neither `owners` nor `repos` is configured

## Migration Notes

- This change is backward compatible
- Existing configurations without `ghes` field continue to work
- Configuration file is required only when using GHES
