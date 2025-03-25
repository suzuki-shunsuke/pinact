package run

import (
	"context"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/pkg/github"
)

type Controller struct {
	repositoriesService RepositoriesService
	fs                  afero.Fs
	cfg                 *Config
	param               *ParamRun
}

func New(ctx context.Context, param *ParamRun) *Controller {
	gh := github.New(ctx)
	return &Controller{
		repositoriesService: &RepositoriesServiceImpl{
			tags:                map[string]*ListTagsResult{},
			releases:            map[string]*ListReleasesResult{},
			commits:             map[string]*GetCommitSHA1Result{},
			RepositoriesService: gh.Repositories,
		},
		param: param,
		fs:    afero.NewOsFs(),
	}
}

func NewController(repoService RepositoriesService, fs afero.Fs) *Controller {
	return &Controller{
		repositoriesService: repoService,
		fs:                  fs,
	}
}
