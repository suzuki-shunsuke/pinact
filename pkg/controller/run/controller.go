package run

import (
	"context"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/pkg/github"
)

type Controller struct {
	repositoriesService RepositoriesService
	fs                  afero.Fs
}

func New(ctx context.Context) *Controller {
	gh := github.New(ctx)
	return &Controller{
		repositoriesService: &RepositoriesServiceImpl{
			tags:                map[string]*ListTagsResult{},
			commits:             map[string]*GetCommitSHA1Result{},
			RepositoriesService: gh.Repositories,
		},
		fs: afero.NewOsFs(),
	}
}

func NewController(repoService RepositoriesService, fs afero.Fs) *Controller {
	return &Controller{
		repositoriesService: repoService,
		fs:                  fs,
	}
}
