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
	ListReleases(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error)
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

type ListReleasesResult struct {
	Releases []*github.RepositoryRelease
	Response *github.Response
	err      error
}

type RepositoriesServiceImpl struct {
	RepositoriesService RepositoriesService
	tags                map[string]*ListTagsResult
	commits             map[string]*GetCommitSHA1Result
	releases            map[string]*ListReleasesResult
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

func (r *RepositoriesServiceImpl) ListReleases(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/%v", owner, repo, opts.Page)
	a, ok := r.releases[key]
	if ok {
		return a.Releases, a.Response, a.err
	}
	releases, resp, err := r.RepositoriesService.ListReleases(ctx, owner, repo, opts)
	r.releases[key] = &ListReleasesResult{
		Releases: releases,
		Response: resp,
		err:      err,
	}
	return releases, resp, err //nolint:wrapcheck
}

func (c *Controller) getLatestVersion(ctx context.Context, logE *logrus.Entry, owner string, repo string) (string, error) {
	lv, err := c.getLatestVersionFromReleases(ctx, logE, owner, repo)
	if err != nil {
		logerr.WithError(logE, err).Debug("get the latest version from releases")
	}
	if lv != "" {
		return lv, nil
	}
	return c.getLatestVersionFromTags(ctx, logE, owner, repo)
}

func compare(latestSemver *version.Version, latestVersion, tag string) (*version.Version, string, error) {
	v, err := version.NewVersion(tag)
	if err != nil {
		if tag > latestVersion {
			latestVersion = tag
		}
		return latestSemver, latestVersion, fmt.Errorf("parse a tag as a semver: %w", err)
	}
	if latestSemver != nil {
		if v.GreaterThan(latestSemver) {
			return latestSemver, "", nil
		}
		return latestSemver, "", nil
	}
	return v, "", nil
}

func (c *Controller) getLatestVersionFromReleases(ctx context.Context, logE *logrus.Entry, owner string, repo string) (string, error) {
	opts := &github.ListOptions{
		PerPage: 30, //nolint:mnd
	}
	releases, _, err := c.repositoriesService.ListReleases(ctx, owner, repo, opts)
	if err != nil {
		return "", fmt.Errorf("list releases: %w", err)
	}
	var latestSemver *version.Version
	latestVersion := ""
	for _, release := range releases {
		tag := release.GetTagName()
		ls, lv, err := compare(latestSemver, latestVersion, tag)
		latestSemver = ls
		latestVersion = lv
		if err != nil {
			logerr.WithError(logE, err).WithField("tag", tag).Debug("compare tags")
			continue
		}
	}
	if latestSemver != nil {
		return latestSemver.Original(), nil
	}
	return latestVersion, nil
}

func (c *Controller) getLatestVersionFromTags(ctx context.Context, logE *logrus.Entry, owner string, repo string) (string, error) {
	opts := &github.ListOptions{
		PerPage: 30, //nolint:mnd
	}
	tags, _, err := c.repositoriesService.ListTags(ctx, owner, repo, opts)
	if err != nil {
		return "", fmt.Errorf("list tags: %w", err)
	}
	var latestSemver *version.Version
	latestVersion := ""
	for _, tag := range tags {
		t := tag.GetName()
		ls, lv, err := compare(latestSemver, latestVersion, t)
		latestSemver = ls
		latestVersion = lv
		if err != nil {
			logerr.WithError(logE, err).WithField("tag", tag).Debug("compare tags")
			continue
		}
	}
	if latestSemver != nil {
		return latestSemver.Original(), nil
	}
	return latestVersion, nil
}
