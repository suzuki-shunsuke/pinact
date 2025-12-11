# Implementation Plan: GitHub Enterprise Server (GHES) Support

## Overview

This document outlines the implementation plan for adding GHES support to pinact.

## Key Design Decisions

### 1. Single GHES Instance

pinact supports one GHES instance in addition to github.com:
- One client for github.com (default)
- One client for the configured GHES instance

### 2. Action Routing

Actions are routed to GHES or github.com based on regex pattern matching in the configuration file.

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
    GHES          *GHES           `json:"ghes,omitempty" yaml:"ghes"`  // Changed from []*GHES
}

type GHES struct {
    BaseURL        string   `json:"base_url" yaml:"base_url"`  // Changed from Host
    Actions        []string `json:"actions"`
    actionPatterns []*regexp.Regexp
}
```

Update GHES methods:

```go
func (g *GHES) Init() error {
    // Validate base_url
    // Compile action patterns as regular expressions
}

func (g *GHES) Match(actionName string) bool {
    // Check if actionName matches any pattern
}
```

### Step 2: Update Token Retrieval

**File:** `pkg/github/github.go`

Replace `GetGHESToken(host string)` with `GetGHESToken()`:

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

Simplify to support single GHES:

```go
type ClientRegistry struct {
    defaultClient *Client
    ghesClient    *Client  // Changed from map[string]*Client
    ghesConfig    *config.GHES
}

func NewClientRegistry(ctx context.Context, defaultClient *Client, ghes *config.GHES) (*ClientRegistry, error) {
    // Create GHES client if config exists
}

func (r *ClientRegistry) ResolveHost(actionName string) bool {
    // Returns true if action should use GHES
}
```

### Step 4: Simplify Controller

**File:** `pkg/controller/run/controller.go`

Simplify GHES service fields:

```go
type Controller struct {
    // Existing fields...
    ghesRepoService RepositoriesService  // Changed from map
    ghesGitService  *GitServiceImpl      // Changed from map
    clientRegistry  ClientRegistry
}

func (c *Controller) getRepositoriesService(actionName string) RepositoriesService {
    if c.clientRegistry != nil && c.clientRegistry.ResolveHost(actionName) {
        return c.ghesRepoService
    }
    return c.repositoriesService
}
```

### Step 5: Update CLI Integration

**File:** `pkg/cli/run/command.go`

Update GHES initialization:

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

### Step 6: Update JSON Schema

**File:** `json-schema/pinact.json`

Change `ghes` from array to object:

```json
"ghes": {
  "type": "object",
  "properties": {
    "base_url": {
      "type": "string",
      "description": "Base URL of the GHES instance"
    },
    "actions": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Regular expression patterns to match action names"
    }
  },
  "required": ["base_url", "actions"]
}
```

## File Change Summary

| File | Change Type | Description |
|------|-------------|-------------|
| `pkg/config/config.go` | Modify | Change GHES from array to single object, host → base_url |
| `pkg/github/github.go` | Modify | Simplify GetGHESToken to check multiple env vars |
| `pkg/github/registry.go` | Modify | Simplify to single GHES client |
| `pkg/controller/run/controller.go` | Modify | Simplify to single GHES service |
| `pkg/cli/run/command.go` | Modify | Update initialization for single GHES |
| `json-schema/pinact.json` | Modify | Change ghes from array to object |

## Error Handling

1. **Missing GHES token**: Return clear error message listing expected environment variables
2. **Invalid regex pattern**: Report error during config initialization with pattern details
3. **GHES API failure**: Return error without fallback to github.com
4. **Invalid base_url**: Validate URL format during config initialization

## Migration Notes

- This change is backward compatible
- Existing configurations without `ghes` field continue to work
- Configuration file is required only when using GHES
