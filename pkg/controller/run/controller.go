package run

import (
	"github.com/spf13/afero"
)

type Controller struct {
	repositoriesService RepositoriesService
	fs                  afero.Fs
	cfg                 *Config
	param               *ParamRun
}

func New(repositoriesService RepositoriesService, fs afero.Fs, param *ParamRun) *Controller {
	return &Controller{
		repositoriesService: repositoriesService,
		param:               param,
		fs:                  fs,
	}
}
