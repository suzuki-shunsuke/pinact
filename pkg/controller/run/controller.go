package run

import (
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/pkg/config"
)

type Controller struct {
	repositoriesService RepositoriesService
	fs                  afero.Fs
	cfg                 *config.Config
	param               *ParamRun
	cfgFinder           ConfigFinder
	cfgReader           ConfigReader
}

type ConfigFinder interface {
	Find(configFilePath string) (string, error)
}

type ConfigReader interface {
	Read(cfg *config.Config, configFilePath string) error
}

func New(repositoriesService RepositoriesService, fs afero.Fs, cfgFinder ConfigFinder, cfgReader ConfigReader, param *ParamRun) *Controller {
	return &Controller{
		repositoriesService: repositoriesService,
		param:               param,
		fs:                  fs,
		cfgFinder:           cfgFinder,
		cfgReader:           cfgReader,
		cfg:                 &config.Config{},
	}
}
