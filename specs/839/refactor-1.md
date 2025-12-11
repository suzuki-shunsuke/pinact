# Refactoring Plan: Service Layer GHES Integration

## Overview

Move GHES client and owner matching logic into service implementations (GitServiceImpl, RepositoriesServiceImpl). Methods automatically select the appropriate client based on the owner parameter. This removes duplicate fields and switching logic from the Controller.

## Current Architecture

```
Controller
├── repositoriesService     (github.com)
├── ghesRepoService         (GHES)
├── gitService              (github.com)
├── ghesGitService          (GHES)
├── pullRequestsService     (github.com)
├── ghesPullRequestsService (GHES)
└── clientRegistry          (owner → host resolution)
```

Callers must call `c.getRepositoriesService(owner)` etc. to get the appropriate service.

## Target Architecture

```
Controller
├── repositoriesService  (internally selects github.com/GHES)
├── gitService           (internally selects github.com/GHES)
└── pullRequestsService  (internally selects github.com/GHES)
```

Each service automatically selects the appropriate client based on owner.

## Changes

### 1. pkg/controller/run/github.go

**GitServiceImpl:**
```go
type GitServiceImpl struct {
    defaultGitService GitService      // github.com
    ghesGitService    GitService      // GHES (nil if not configured)
    ghesConfig        *config.GHES
    Commits           map[string]*GetCommitResult
}

func (g *GitServiceImpl) isGHES(owner string) bool {
    return g.ghesConfig != nil && g.ghesConfig.Match(owner)
}

func (g *GitServiceImpl) getService(owner string) GitService {
    if g.isGHES(owner) && g.ghesGitService != nil {
        return g.ghesGitService
    }
    return g.defaultGitService
}

func (g *GitServiceImpl) GetCommit(ctx context.Context, owner, repo, sha string) (*github.Commit, *github.Response, error) {
    key := fmt.Sprintf("%s/%s/%s", owner, repo, sha)
    if result, ok := g.Commits[key]; ok {
        return result.Commit, result.Response, result.err
    }
    service := g.getService(owner)
    commit, resp, err := service.GetCommit(ctx, owner, repo, sha)
    g.Commits[key] = &GetCommitResult{Commit: commit, Response: resp, err: err}
    return commit, resp, err
}
```

**RepositoriesServiceImpl:**
```go
type RepositoriesServiceImpl struct {
    defaultRepoService RepositoriesService
    ghesRepoService    RepositoriesService
    ghesConfig         *config.GHES
    Tags               map[string]*ListTagsResult
    Commits            map[string]*GetCommitSHA1Result
    Releases           map[string]*ListReleasesResult
}

func (r *RepositoriesServiceImpl) isGHES(owner string) bool {
    return r.ghesConfig != nil && r.ghesConfig.Match(owner)
}

func (r *RepositoriesServiceImpl) getService(owner string) RepositoriesService {
    if r.isGHES(owner) && r.ghesRepoService != nil {
        return r.ghesRepoService
    }
    return r.defaultRepoService
}
```

**PullRequestsServiceImpl (new):**
```go
type PullRequestsServiceImpl struct {
    defaultPRService PullRequestsService
    ghesPRService    PullRequestsService
    ghesConfig       *config.GHES
}

func (p *PullRequestsServiceImpl) CreateComment(ctx context.Context, owner, repo string, number int, comment *github.PullRequestComment) (*github.PullRequestComment, *github.Response, error) {
    if p.ghesConfig != nil && p.ghesConfig.Match(owner) && p.ghesPRService != nil {
        return p.ghesPRService.CreateComment(ctx, owner, repo, number, comment)
    }
    return p.defaultPRService.CreateComment(ctx, owner, repo, number, comment)
}
```

### 2. pkg/controller/run/controller.go

**Remove:**
- `ghesRepoService`, `ghesGitService`, `ghesPullRequestsService` fields
- `clientRegistry` field
- `ClientRegistry` interface
- `SetClientRegistry()`, `SetGHESServices()` methods
- `getRepositoriesService()`, `getGitService()`, `getPullRequestsService()` methods

### 3. pkg/controller/run/parse_line.go, github.go

**Before:**
```go
repoService := c.getRepositoriesService(action.RepoOwner)
sha, _, err := repoService.GetCommitSHA1(ctx, ...)
```

**After:**
```go
sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, ...)
```

### 4. pkg/cli/run/command.go

```go
// Get GHES config
ghesConfig := cfg.GHES
if ghesConfig == nil {
    ghesConfig = config.GHESFromEnv()
}

var ghesRepoService run.RepositoriesService
var ghesGitService run.GitService
var ghesPRService run.PullRequestsService

if ghesConfig != nil {
    if err := ghesConfig.Init(); err != nil {
        return fmt.Errorf("initialize GHES config: %w", err)
    }
    registry, err := github.NewClientRegistry(ctx, gh, ghesConfig)
    if err != nil {
        return fmt.Errorf("create GitHub client registry: %w", err)
    }
    client := registry.GetGHESClient()
    ghesRepoService = client.Repositories
    ghesGitService = client.Git
    ghesPRService = client.PullRequests
}

repoService := &run.RepositoriesServiceImpl{
    defaultRepoService: gh.Repositories,
    ghesRepoService:    ghesRepoService,
    ghesConfig:         ghesConfig,
    Tags:               map[string]*run.ListTagsResult{},
    Releases:           map[string]*run.ListReleasesResult{},
    Commits:            map[string]*run.GetCommitSHA1Result{},
}

gitService := &run.GitServiceImpl{
    defaultGitService: gh.Git,
    ghesGitService:    ghesGitService,
    ghesConfig:        ghesConfig,
    Commits:           map[string]*run.GetCommitResult{},
}

prService := &run.PullRequestsServiceImpl{
    defaultPRService: gh.PullRequests,
    ghesPRService:    ghesPRService,
    ghesConfig:       ghesConfig,
}

ctrl := run.New(repoService, prService, gitService, ...)
```

## Files to Modify

| File | Change |
|------|--------|
| `pkg/controller/run/github.go` | Add GHES fields to service impl, internal client selection |
| `pkg/controller/run/controller.go` | Remove GHES fields and methods |
| `pkg/controller/run/parse_line.go` | Remove `getXxxService(owner)` calls |
| `pkg/cli/run/command.go` | Update initialization code |

## Implementation Order

1. `github.go` - Update service impl structs and methods
2. `controller.go` - Remove unnecessary fields and methods
3. `parse_line.go` - Update call sites
4. `command.go` - Update initialization code
5. Run tests and fix
