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
