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
	fs                  afero.Fs
	cfg                 *config.Config
	param               *ParamRun
	cfgFinder           ConfigFinder
	cfgReader           ConfigReader
	logger              *Logger
}

type ConfigFinder interface {
	Find(configFilePath string) (string, error)
}

type ConfigReader interface {
	Read(cfg *config.Config, configFilePath string) error
}

// New creates a new Controller for running pinact operations.
// It initializes the controller with all necessary dependencies for processing
// GitHub Actions workflow files, including GitHub API services, filesystem
// interface, configuration management, and operation parameters.
//
// Parameters:
//   - repositoriesService: GitHub API service for repository operations
//   - pullRequestsService: GitHub API service for pull request operations
//   - fs: filesystem interface for file operations
//   - cfgFinder: service for locating configuration files
//   - cfgReader: service for reading and parsing configuration files
//   - param: operation parameters and settings
//
// Returns a pointer to the configured Controller.
func New(repositoriesService RepositoriesService, pullRequestsService PullRequestsService, fs afero.Fs, cfgFinder ConfigFinder, cfgReader ConfigReader, param *ParamRun) *Controller {
	return &Controller{
		repositoriesService: repositoriesService,
		pullRequestsService: pullRequestsService,
		param:               param,
		fs:                  fs,
		cfgFinder:           cfgFinder,
		cfgReader:           cfgReader,
		cfg:                 &config.Config{},
		logger:              NewLogger(param.Stderr),
	}
}
