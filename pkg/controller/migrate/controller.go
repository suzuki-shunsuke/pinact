package migrate

import (
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/pkg/config"
)

type Controller struct {
	fs        afero.Fs
	cfg       *config.Config
	param     *Param
	cfgFinder ConfigFinder
	cfgReader ConfigReader
}

type ConfigFinder interface {
	Find(configFilePath string) (string, error)
}

type ConfigReader interface {
	Read(cfg *config.Config, configFilePath string) error
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
