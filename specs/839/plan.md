# Implementation Plan: GitHub Enterprise Server (GHES) Support

## Overview

This document outlines the implementation plan for adding GHES support to pinact.

## Key Design Decisions

### 1. Multiple GitHub Clients

Instead of a single GitHub client, pinact will manage multiple clients:
- One for github.com (default)
- One for each configured GHES instance

### 2. Action Routing

Actions are routed to the appropriate GitHub instance based on regex pattern matching in the configuration file.

### 3. Token Management

Tokens are retrieved from environment variables using a naming convention:
- `GITHUB_TOKEN` for github.com
- `GITHUB_TOKEN_<host>` for GHES (e.g., `GITHUB_TOKEN_ghes_example_com`)

## Implementation Steps

### Step 1: Extend Configuration Schema

**File:** `pkg/config/config.go`

Add GHES configuration to the Config struct:

```go
type Config struct {
    Version       int             `json:"version,omitempty" jsonschema:"enum=2,enum=3"`
    Files         []*File         `json:"files,omitempty"`
    IgnoreActions []*IgnoreAction `json:"ignore_actions,omitempty" yaml:"ignore_actions"`
    GHES          []*GHES         `json:"ghes,omitempty" yaml:"ghes"`
}

type GHES struct {
    Host    string   `json:"host"`
    Actions []string `json:"actions"`
    // Compiled regular expressions (not serialized)
    actionPatterns []*regexp.Regexp
}
```

Add initialization and matching methods for GHES:

```go
func (g *GHES) Init() error {
    // Validate host
    // Compile action patterns as regular expressions
}

func (g *GHES) Match(actionName string) bool {
    // Check if actionName matches any pattern
}
```

### Step 2: Create GitHub Client Registry

**File:** `pkg/github/registry.go` (new file)

Create a registry to manage multiple GitHub clients:

```go
type ClientRegistry struct {
    defaultClient *Client
    ghesClients   map[string]*Client  // key: host
}

func NewClientRegistry(ctx context.Context, logger *slog.Logger, ghesConfigs []*config.GHES) (*ClientRegistry, error) {
    // Create default client for github.com
    // Create clients for each GHES instance
}

func (r *ClientRegistry) GetClient(host string) *Client {
    // Return GHES client if host is specified, otherwise default
}
```

### Step 3: Modify GitHub Client Creation

**File:** `pkg/github/github.go`

Add function to create client with custom base URL:

```go
func NewWithBaseURL(ctx context.Context, logger *slog.Logger, baseURL, token string) *Client {
    httpClient := getHTTPClientForGitHub(ctx, logger, token)
    client := github.NewClient(httpClient)
    if baseURL != "" {
        // Set client.BaseURL to baseURL + "/api/v3"
        var err error
        client.BaseURL, err = url.Parse(baseURL + "/api/v3/")
        if err != nil {
            // handle error
        }
    }
    return client
}
```

Add function to get GHES token from environment:

```go
func getGHESToken(host string) string {
    // Convert host to env var name: ghes.example.com -> ghes_example_com
    envName := "GITHUB_TOKEN_" + strings.ReplaceAll(strings.ReplaceAll(host, ".", "_"), "-", "_")
    return os.Getenv(envName)
}
```

### Step 4: Add Host Resolution to Controller

**File:** `pkg/controller/run/controller.go`

Modify Controller to use ClientRegistry:

```go
type Controller struct {
    // Existing fields...
    clientRegistry *ClientRegistry
    ghesConfigs    []*config.GHES
}
```

Add method to resolve host for an action:

```go
func (c *Controller) resolveHost(actionName string) string {
    for _, ghes := range c.ghesConfigs {
        if ghes.Match(actionName) {
            return ghes.Host
        }
    }
    return "" // empty means github.com
}
```

### Step 5: Create Service Wrappers with Host Awareness

**File:** `pkg/controller/run/github.go`

Modify or create service implementations that select the correct client based on action:

```go
type MultiHostRepositoriesService struct {
    registry    *ClientRegistry
    ghesConfigs []*config.GHES
    cache       map[string]*RepositoriesServiceImpl  // per-host cache
}

func (m *MultiHostRepositoriesService) GetServiceForAction(actionName string) RepositoriesService {
    host := m.resolveHost(actionName)
    // Return cached service or create new one
}
```

### Step 6: Update Action Processing

**File:** `pkg/controller/run/parse_line.go`

Modify methods that call GitHub API to use the correct service:

```go
func (c *Controller) parseNoTagLine(ctx context.Context, logger *slog.Logger, action *Action) (string, error) {
    // Get the appropriate service based on action name
    repoService := c.getRepositoriesService(action.Name)

    // Use repoService instead of c.repositoriesService
    sha, _, err := repoService.GetCommitSHA1(ctx, action.RepoOwner, action.RepoName, lv, "")
    // ...
}
```

### Step 7: Update CLI Integration

**File:** `pkg/cli/run/command.go`

Update the action function to initialize the client registry:

```go
func (r *runner) action(ctx context.Context, logger *slogutil.Logger, flags *Flags) error {
    // ... existing code ...

    // Read config first to get GHES settings
    cfg := &config.Config{}
    cfgFinder := config.NewFinder(fs)
    cfgReader := config.NewReader(fs)
    configPath, _ := cfgFinder.Find(flags.Config)
    if err := cfgReader.Read(cfg, configPath); err != nil {
        return err
    }

    // Create client registry with GHES configs
    registry, err := github.NewClientRegistry(ctx, logger.Logger, cfg.GHES)
    if err != nil {
        return fmt.Errorf("create GitHub client registry: %w", err)
    }

    // Pass registry to controller
    ctrl := run.New(registry, /* ... */)
    // ...
}
```

### Step 8: Update JSON Schema

**File:** `json-schema/pinact.json`

Add GHES configuration to the JSON schema for validation and IDE support.

### Step 9: Add Tests

Create test files for:
- `pkg/config/config_test.go` - GHES config parsing and matching
- `pkg/github/registry_test.go` - Client registry functionality
- `pkg/controller/run/parse_line_test.go` - Action routing tests

### Step 10: Update Documentation

- Update README.md with GHES configuration examples
- Add documentation for environment variables
- Add error code documentation for GHES-related errors

## File Change Summary

| File | Change Type | Description |
|------|-------------|-------------|
| `pkg/config/config.go` | Modify | Add GHES struct and Config.GHES field |
| `pkg/github/github.go` | Modify | Add NewWithBaseURL and getGHESToken functions |
| `pkg/github/registry.go` | New | Client registry for managing multiple GitHub clients |
| `pkg/controller/run/controller.go` | Modify | Add clientRegistry and host resolution |
| `pkg/controller/run/github.go` | Modify | Add multi-host service wrappers |
| `pkg/controller/run/parse_line.go` | Modify | Use host-aware services for API calls |
| `pkg/cli/run/command.go` | Modify | Initialize client registry with GHES configs |
| `json-schema/pinact.json` | Modify | Add GHES schema definition |

## Error Handling

1. **Missing GHES token**: Return clear error message indicating which environment variable is expected
2. **Invalid regex pattern**: Report error during config initialization with pattern details
3. **GHES API failure**: Return error without fallback to github.com
4. **Invalid GHES host**: Validate host format during config initialization

## Testing Strategy

1. **Unit tests**: Test each component in isolation
   - Config parsing and GHES matching
   - Token retrieval from environment
   - Client creation with custom base URL

2. **Integration tests**: Test end-to-end flows
   - Action routing to correct host
   - API calls to mock GHES server

3. **Manual testing**: Test with real GHES instance if available

## Migration Notes

- This change is backward compatible
- Existing configurations without `ghes` field continue to work
- Configuration file is required only when using GHES
