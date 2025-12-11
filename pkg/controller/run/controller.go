// Package run implements the core business logic for pinning GitHub Actions.
// This package contains the main controller that orchestrates the entire pinning process,
// including parsing workflow files, resolving action versions through GitHub API,
// converting mutable tags to immutable commit SHAs, and applying updates.
// It handles various operation modes (check, diff, fix, update), manages caching
// for API efficiency, and supports creating pull request reviews. The package
// provides a clean separation between the CLI layer and the actual file processing
// logic, coordinating with GitHub services and filesystem operations.
package run

import (
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
)

type Controller struct {
	repositoriesService RepositoriesService
	pullRequestsService PullRequestsService
	gitService          *GitServiceImpl
	fs                  afero.Fs
	cfg                 *config.Config
	param               *ParamRun
	cfgFinder           ConfigFinder
	cfgReader           ConfigReader
	logger              *Logger
	// GHES support
	ghesRepoServices map[string]RepositoriesService // key: host
	ghesGitServices  map[string]*GitServiceImpl     // key: host
	clientRegistry   ClientRegistry
}

type ConfigFinder interface {
	Find(configFilePath string) (string, error)
}

type ConfigReader interface {
	Read(cfg *config.Config, configFilePath string) error
}

type ClientRegistry interface {
	ResolveHost(actionName string) string
}

// New creates a new Controller for running pinact operations.
// It initializes the controller with all necessary dependencies for processing
// GitHub Actions workflow files, including GitHub API services, filesystem
// interface, configuration management, and operation parameters.
//
// Parameters:
//   - repositoriesService: GitHub API service for repository operations
//   - pullRequestsService: GitHub API service for pull request operations
//   - gitService: GitHub API service for git operations (optional, for cooldown feature)
//   - fs: filesystem interface for file operations
//   - cfgFinder: service for locating configuration files
//   - cfgReader: service for reading and parsing configuration files
//   - param: operation parameters and settings
//
// Returns a pointer to the configured Controller.
func New(repositoriesService RepositoriesService, pullRequestsService PullRequestsService, gitService *GitServiceImpl, fs afero.Fs, cfgFinder ConfigFinder, cfgReader ConfigReader, param *ParamRun) *Controller {
	return &Controller{
		repositoriesService: repositoriesService,
		pullRequestsService: pullRequestsService,
		gitService:          gitService,
		param:               param,
		fs:                  fs,
		cfgFinder:           cfgFinder,
		cfgReader:           cfgReader,
		cfg:                 &config.Config{},
		logger:              NewLogger(param.Stderr),
		ghesRepoServices:    make(map[string]RepositoriesService),
		ghesGitServices:     make(map[string]*GitServiceImpl),
	}
}

// SetClientRegistry sets the client registry for GHES support.
// This must be called after New() to enable GHES functionality.
//
// Parameters:
//   - registry: client registry for resolving hosts
func (c *Controller) SetClientRegistry(registry ClientRegistry) {
	c.clientRegistry = registry
}

// SetGHESServices sets the GitHub services for a specific GHES host.
//
// Parameters:
//   - host: hostname of the GHES instance
//   - repoService: repository service for the GHES instance
//   - gitService: git service for the GHES instance
func (c *Controller) SetGHESServices(host string, repoService RepositoriesService, gitService *GitServiceImpl) {
	c.ghesRepoServices[host] = repoService
	c.ghesGitServices[host] = gitService
}

// getRepositoriesService returns the appropriate repositories service for an action.
// It resolves the host based on the action name and returns the corresponding service.
//
// Parameters:
//   - actionName: action name to get service for (format: owner/repo)
//
// Returns the repositories service for the action's host.
func (c *Controller) getRepositoriesService(actionName string) RepositoriesService {
	if c.clientRegistry == nil {
		return c.repositoriesService
	}
	host := c.clientRegistry.ResolveHost(actionName)
	if host == "" {
		return c.repositoriesService
	}
	if svc, ok := c.ghesRepoServices[host]; ok {
		return svc
	}
	return c.repositoriesService
}

// getGitService returns the appropriate git service for an action.
// It resolves the host based on the action name and returns the corresponding service.
//
// Parameters:
//   - actionName: action name to get service for (format: owner/repo)
//
// Returns the git service for the action's host.
func (c *Controller) getGitService(actionName string) *GitServiceImpl {
	if c.clientRegistry == nil {
		return c.gitService
	}
	host := c.clientRegistry.ResolveHost(actionName)
	if host == "" {
		return c.gitService
	}
	if svc, ok := c.ghesGitServices[host]; ok {
		return svc
	}
	return c.gitService
}
