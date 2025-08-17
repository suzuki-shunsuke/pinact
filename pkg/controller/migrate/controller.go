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

func New(fs afero.Fs, cfgFinder ConfigFinder, param *Param) *Controller {
	return &Controller{
		param:     param,
		fs:        fs,
		cfg:       &config.Config{},
		cfgFinder: cfgFinder,
	}
}
