# Implementation Plan: GitHub Enterprise Server (GHES) Support

## Overview

This document outlines the implementation plan for adding GHES support to pinact.

## Key Design Decisions

### 1. Single GHES Instance

pinact supports one GHES instance in addition to github.com:
- One client for github.com (default)
- One client for the configured GHES instance

### 2. Repository Routing

Repositories are routed to GHES or github.com based on `owners` (exact match against the repository owner).

### 3. Configuration Sources

GHES can be configured via:
1. Configuration file (`.pinact.yaml`)
2. Environment variables (when config file does not have `ghes` settings)

Environment variables for GHES configuration:
- `PINACT_GHES_BASE_URL`: GHES base URL (e.g., `https://ghes.example.com`)
- `PINACT_GHES_OWNERS`: Comma-separated list of repository owners
- `GITHUB_API_URL`: Alternative to `PINACT_GHES_BASE_URL` (commonly set in GitHub Actions on GHES)

Resolution priority for base URL:
1. If `PINACT_GHES_BASE_URL` is set, it is used (and `GITHUB_API_URL` is ignored)
2. If `PINACT_GHES_BASE_URL` is not set but `GITHUB_API_URL` is set, `GITHUB_API_URL` is used

Requirements:
- If `PINACT_GHES_BASE_URL` is set, `PINACT_GHES_OWNERS` is required
- If `PINACT_GHES_OWNERS` is set, either `PINACT_GHES_BASE_URL` or `GITHUB_API_URL` is required
- If neither `PINACT_GHES_BASE_URL` nor `GITHUB_API_URL` is set, `PINACT_GHES_OWNERS` is optional (only github.com actions are processed)

### 4. Token Management

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
    BaseURL string   `json:"base_url" yaml:"base_url"`
    Owners  []string `json:"owners"`
}
```

Update GHES methods:

```go
func (g *GHES) Init() error {
    // Validate base_url is not empty
    // Validate owners is not empty
}

func (g *GHES) Match(owner string) bool {
    // Check if owner matches any entry in Owners (exact match)
}
```

Add function to create GHES config from environment variables:

```go
func GHESFromEnv() *GHES {
    // Get base URL from PINACT_GHES_BASE_URL or GITHUB_API_URL
    baseURL := os.Getenv("PINACT_GHES_BASE_URL")
    if baseURL == "" {
        baseURL = os.Getenv("GITHUB_API_URL")
    }

    // Get owners from PINACT_GHES_OWNERS
    ownersStr := os.Getenv("PINACT_GHES_OWNERS")

    // If neither base URL nor owners, return nil
    if baseURL == "" && ownersStr == "" {
        return nil
    }

    // Parse owners (comma-separated)
    var owners []string
    if ownersStr != "" {
        owners = strings.Split(ownersStr, ",")
    }

    return &GHES{
        BaseURL: baseURL,
        Owners:  owners,
    }
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

func (r *ClientRegistry) ResolveHost(owner string) bool {
    // Returns true if owner should use GHES
}
```

### Step 4: Simplify Controller

**File:** `pkg/controller/run/controller.go`

```go
type Controller struct {
    // Existing fields...
    ghesRepoService         RepositoriesService
    ghesGitService          *GitServiceImpl
    ghesPullRequestsService PullRequestsService
    clientRegistry          ClientRegistry
}

type ClientRegistry interface {
    ResolveHost(owner string) bool
}

func (c *Controller) getRepositoriesService(owner string) RepositoriesService {
    if c.clientRegistry != nil && c.clientRegistry.ResolveHost(owner) {
        return c.ghesRepoService
    }
    return c.repositoriesService
}

func (c *Controller) getGitService(owner string) *GitServiceImpl {
    if c.clientRegistry != nil && c.clientRegistry.ResolveHost(owner) {
        return c.ghesGitService
    }
    return c.gitService
}

func (c *Controller) getPullRequestsService(owner string) PullRequestsService {
    if c.clientRegistry != nil && c.clientRegistry.ResolveHost(owner) {
        return c.ghesPullRequestsService
    }
    return c.pullRequestsService
}
```

### Step 5: Update parse_line.go

**File:** `pkg/controller/run/parse_line.go`

Remove `actionName` parameter from `getLatestVersion` and related methods. Instead, get the appropriate service once at the beginning of each method using the action's repository name.

```go
func (c *Controller) parseNoTagLine(ctx context.Context, logger *slog.Logger, action *Action) (string, error) {
    repoService := c.getRepositoriesService(action.RepoOwner)
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
// Get GHES config from config file or environment variables
ghesConfig := cfg.GHES
if ghesConfig == nil {
    ghesConfig = config.GHESFromEnv()
}

// Set up GHES support if configured
if ghesConfig != nil {
    if err := ghesConfig.Init(); err != nil {
        return fmt.Errorf("initialize GHES config: %w", err)
    }
    registry, err := github.NewClientRegistry(ctx, gh, ghesConfig)
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
    }
  },
  "required": ["base_url", "owners"]
}
```

## File Change Summary

| File | Change Type | Description |
|------|-------------|-------------|
| `pkg/config/config.go` | Modify | Add owners field, update Match to check owners, add GHESFromEnv() |
| `pkg/github/github.go` | Modify | Simplify GetGHESToken to check multiple env vars |
| `pkg/github/registry.go` | Modify | Simplify to single GHES client |
| `pkg/controller/run/controller.go` | Modify | Simplify to single GHES service, add GHES PullRequestsService for review mode |
| `pkg/controller/run/github.go` | Modify | Remove actionName parameter from version methods, update review to use appropriate PullRequestsService |
| `pkg/controller/run/parse_line.go` | Modify | Get service once per method |
| `pkg/cli/run/command.go` | Modify | Update initialization for single GHES, support env vars, set GHES PullRequestsService |
| `json-schema/pinact.json` | Modify | Add owners field (required) |

## Error Handling

1. **Missing GHES token**: Return clear error message listing expected environment variables
2. **GHES API failure**: Return error without fallback to github.com
3. **Missing base_url when owners set**: Return error during config initialization
4. **Missing owners when base_url set**: Return error during config initialization

## Migration Notes

- This change is backward compatible
- Existing configurations without `ghes` field continue to work
- GHES can be configured via config file or environment variables
