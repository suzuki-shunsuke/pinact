package run

import (
	"context"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/pkg/github"
)

type Controller struct {
	repositoriesService RepositoriesService
	fs                  afero.Fs
	update              bool
}

type InputNew struct {
	Update bool
}

func New(ctx context.Context, input *InputNew) *Controller {
	gh := github.New(ctx)
	return &Controller{
		repositoriesService: &RepositoriesServiceImpl{
			tags:                map[string]*ListTagsResult{},
			releases:            map[string]*ListReleasesResult{},
			commits:             map[string]*GetCommitSHA1Result{},
			RepositoriesService: gh.Repositories,
		},
		fs:     afero.NewOsFs(),
		update: input.Update,
	}
}

func NewController(repoService RepositoriesService, fs afero.Fs) *Controller {
	return &Controller{
		repositoriesService: repoService,
		fs:                  fs,
	}
}
