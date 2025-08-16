// Package migrate handles configuration file migration between schema versions.
// This package provides the business logic for upgrading pinact configuration files
// when new schema versions are introduced. It ensures smooth transitions between
// different configuration formats, preserving user settings while adapting to new
// features and structural changes. The migration process maintains backward
// compatibility and helps users keep their configurations up-to-date with the
// latest pinact capabilities.
package migrate

import (
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
)

type Controller struct {
	fs        afero.Fs
	cfg       *config.Config
	param     *Param
	cfgFinder ConfigFinder
}

type ConfigFinder interface {
	Find(configFilePath string) (string, error)
}

type Param struct {
	ConfigFilePath string
}

// New creates a new Controller for configuration migration operations.\n// It initializes the controller with filesystem interface, configuration finder,\n// and migration parameters needed to perform schema migrations.\n//\n// Parameters:\n//   - fs: filesystem interface for file operations\n//   - cfgFinder: service for locating configuration files\n//   - param: migration parameters including configuration file path\n//\n// Returns a pointer to the configured Controller.\nfunc New(fs afero.Fs, cfgFinder ConfigFinder, param *Param) *Controller {
	return &Controller{
		param:     param,
		fs:        fs,
		cfg:       &config.Config{},
		cfgFinder: cfgFinder,
	}
}
