package run

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
	"github.com/suzuki-shunsuke/pinact/pkg/github"
)

type RepositoriesService interface {
	ListTags(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error)
	GetCommitSHA1(ctx context.Context, owner, repo, ref, lastSHA string) (string, *github.Response, error)
}

func (r *RepositoriesServiceImpl) GetCommitSHA1(ctx context.Context, owner, repo, ref, lastSHA string) (string, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/%s", owner, repo, ref)
	a, ok := r.commits[key]
	if ok {
		return a.SHA, a.Response, a.err
	}
	sha, resp, err := r.RepositoriesService.GetCommitSHA1(ctx, owner, repo, ref, lastSHA)
	r.commits[key] = &GetCommitSHA1Result{
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

func (r *RepositoriesServiceImpl) ListTags(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/%v", owner, repo, opts.Page)
	a, ok := r.tags[key]
	if ok {
		return a.Tags, a.Response, a.err
	}
	tags, resp, err := r.RepositoriesService.ListTags(ctx, owner, repo, opts)
	r.tags[key] = &ListTagsResult{
		Tags:     tags,
		Response: resp,
		err:      err,
	}
	return tags, resp, err //nolint:wrapcheck
}

func (c *Controller) GetLatestVersion(ctx context.Context, logE *logrus.Entry, owner string, repo string) (string, *github.Response, error) {
	opts := &github.ListOptions{
		PerPage: 30, //nolint:mnd
	}
	tags, resp, err := c.repositoriesService.ListTags(ctx, owner, repo, opts)
	if err != nil {
		return "", resp, fmt.Errorf("list tags: %w", err)
	}
	var latestSemver *version.Version
	lv := ""
	for _, tag := range tags {
		v, err := version.NewVersion(tag.GetName())
		if err != nil {
			logerr.WithError(logE, err).Warn("parse a version")
		}
		if latestSemver != nil {
			if v.GreaterThan(latestSemver) {
				latestSemver = v
			}
			continue
		}
		latestSemver = v
		if lv == "" {
			lv = tag.GetName()
			continue
		}
		if a := tag.GetName(); a > lv {
			lv = a
		}
	}
	if latestSemver != nil {
		return latestSemver.Original(), resp, nil
	}
	return lv, resp, nil
}
