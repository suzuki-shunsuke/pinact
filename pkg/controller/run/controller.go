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
	ghesRepoService RepositoriesService
	ghesGitService  *GitServiceImpl
	clientRegistry  ClientRegistry
}

type ConfigFinder interface {
	Find(configFilePath string) (string, error)
}

type ConfigReader interface {
	Read(cfg *config.Config, configFilePath string) error
}

type ClientRegistry interface {
	ResolveHost(actionName string) bool
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

// SetGHESServices sets the GitHub services for the GHES instance.
//
// Parameters:
//   - repoService: repository service for the GHES instance
//   - gitService: git service for the GHES instance
func (c *Controller) SetGHESServices(repoService RepositoriesService, gitService *GitServiceImpl) {
	c.ghesRepoService = repoService
	c.ghesGitService = gitService
}

// getRepositoriesService returns the appropriate repositories service for an action.
// It resolves whether the action should use GHES and returns the corresponding service.
//
// Parameters:
//   - actionName: action name to get service for (format: owner/repo)
//
// Returns the repositories service for the action's host.
func (c *Controller) getRepositoriesService(actionName string) RepositoriesService {
	if c.clientRegistry == nil {
		return c.repositoriesService
	}
	if c.clientRegistry.ResolveHost(actionName) && c.ghesRepoService != nil {
		return c.ghesRepoService
	}
	return c.repositoriesService
}

// getGitService returns the appropriate git service for an action.
// It resolves whether the action should use GHES and returns the corresponding service.
//
// Parameters:
//   - actionName: action name to get service for (format: owner/repo)
//
// Returns the git service for the action's host.
func (c *Controller) getGitService(actionName string) *GitServiceImpl {
	if c.clientRegistry == nil {
		return c.gitService
	}
	if c.clientRegistry.ResolveHost(actionName) && c.ghesGitService != nil {
		return c.ghesGitService
	}
	return c.gitService
}
