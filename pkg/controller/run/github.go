package run

import (
	"context"
	"fmt"

	"github.com/suzuki-shunsuke/pinact/pkg/github"
)

type RepositoriesService interface {
	ListTags(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error)
	GetCommitSHA1(ctx context.Context, owner, repo, ref, lastSHA string) (string, *github.Response, error)
}

func (repositoriesService *RepositoriesServiceImpl) GetCommitSHA1(ctx context.Context, owner, repo, ref, lastSHA string) (string, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/%s", owner, repo, ref)
	a, ok := repositoriesService.commits[key]
	if ok {
		return a.SHA, a.Response, a.err
	}
	sha, resp, err := repositoriesService.RepositoriesService.GetCommitSHA1(ctx, owner, repo, ref, lastSHA)
	repositoriesService.commits[key] = &GetCommitSHA1Result{
		SHA:      sha,
		Response: resp,
		err:      err,
	}
	return sha, resp, err //nolint:wrapcheck
}

type ListTagsResult struct {
	Tags     []*github.RepositoryTag
	Response *github.Response
	err      error
}

type RepositoriesServiceImpl struct {
	RepositoriesService RepositoriesService
	tags                map[string]*ListTagsResult
	commits             map[string]*GetCommitSHA1Result
}

type GetCommitSHA1Result struct {
	SHA      string
	Response *github.Response
	err      error
}

func (repositoriesService *RepositoriesServiceImpl) ListTags(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/%v", owner, repo, opts.Page)
	a, ok := repositoriesService.tags[key]
	if ok {
		return a.Tags, a.Response, a.err
	}
	tags, resp, err := repositoriesService.RepositoriesService.ListTags(ctx, owner, repo, opts)
	repositoriesService.tags[key] = &ListTagsResult{
		Tags:     tags,
		Response: resp,
		err:      err,
	}
	return tags, resp, err //nolint:wrapcheck
}
