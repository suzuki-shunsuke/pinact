package run

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
)

type RepositoriesService interface {
	ListTags(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error)
	GetCommitSHA1(ctx context.Context, owner, repo, ref, lastSHA string) (string, *github.Response, error)
	ListReleases(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error)
	GetLatestRelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, *github.Response, error)
}

type PullRequestsService interface {
	CreateComment(ctx context.Context, owner, repo string, number int, comment *github.PullRequestComment) (*github.PullRequestComment, *github.Response, error)
}

// GetCommitSHA1 retrieves the commit SHA for a given reference with caching.
// It first checks the cache and returns cached results if available.
// Otherwise, it calls the underlying service and caches the result.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - owner: repository owner
//   - repo: repository name
//   - ref: reference (tag, branch, or commit)
//   - lastSHA: last known SHA for optimization
//
// Returns the commit SHA, GitHub response, and any error.
func (r *RepositoriesServiceImpl) GetCommitSHA1(ctx context.Context, owner, repo, ref, lastSHA string) (string, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/%s", owner, repo, ref)
	a, ok := r.Commits[key]
	if ok {
		return a.SHA, a.Response, a.err
	}
	sha, resp, err := r.RepositoriesService.GetCommitSHA1(ctx, owner, repo, ref, lastSHA)
	r.Commits[key] = &GetCommitSHA1Result{
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
	Tags                map[string]*ListTagsResult
	Commits             map[string]*GetCommitSHA1Result
	Releases            map[string]*ListReleasesResult
	LatestReleases      map[string]*GetLatestReleaseResult
}

type GetLatestReleaseResult struct {
	Release  *github.RepositoryRelease
	Response *github.Response
	err      error
}

type GetCommitSHA1Result struct {
	SHA      string
	Response *github.Response
	err      error
}

// ListTags retrieves repository tags with caching.
// It first checks the cache and returns cached results if available.
// Otherwise, it calls the underlying service and caches the result.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - owner: repository owner
//   - repo: repository name
//   - opts: GitHub API options for pagination and filtering
//
// Returns repository tags, GitHub response, and any error.
func (r *RepositoriesServiceImpl) ListTags(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/%v", owner, repo, opts.Page)
	a, ok := r.Tags[key]
	if ok {
		return a.Tags, a.Response, a.err
	}
	tags, resp, err := r.RepositoriesService.ListTags(ctx, owner, repo, opts)
	r.Tags[key] = &ListTagsResult{
		Tags:     tags,
		Response: resp,
		err:      err,
	}
	return tags, resp, err //nolint:wrapcheck
}

// ListReleases retrieves repository releases with caching.
// It first checks the cache and returns cached results if available.
// Otherwise, it calls the underlying service and caches the result.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - owner: repository owner
//   - repo: repository name
//   - opts: GitHub API options for pagination and filtering
//
// Returns repository releases, GitHub response, and any error.
func (r *RepositoriesServiceImpl) ListReleases(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/%v", owner, repo, opts.Page)
	a, ok := r.Releases[key]
	if ok {
		return a.Releases, a.Response, a.err
	}
	releases, resp, err := r.RepositoriesService.ListReleases(ctx, owner, repo, opts)
	r.Releases[key] = &ListReleasesResult{
		Releases: releases,
		Response: resp,
		err:      err,
	}
	return releases, resp, err //nolint:wrapcheck
}

func (r *RepositoriesServiceImpl) GetLatestRelease(ctx context.Context, owner, repo string) (*github.RepositoryRelease, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/latest", owner, repo)
	a, ok := r.LatestReleases[key]
	if ok {
		return a.Release, a.Response, a.err
	}
	release, resp, err := r.RepositoriesService.GetLatestRelease(ctx, owner, repo)
	r.LatestReleases[key] = &GetLatestReleaseResult{
		Release:  release,
		Response: resp,
		err:      err,
	}
	return release, resp, err //nolint:wrapcheck
}

// getLatestVersion determines the latest version of a repository.
// It first tries to get the latest version from releases, and if that fails
// or returns empty, it falls back to getting the latest version from tags.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - logE: logrus entry for structured logging
//   - owner: repository owner
//   - repo: repository name
//   - currentVersion: current version to check if stable (empty string to include all versions)
//
// Returns the latest version string or an error.
func (c *Controller) getLatestVersion(ctx context.Context, logE *logrus.Entry, owner string, repo string, currentVersion string) (string, error) {
	lv, err := c.getLatestVersionFromReleases(ctx, logE, owner, repo, currentVersion)
	if err != nil {
		logerr.WithError(logE, err).Debug("get the latest version from releases")
	}
	if lv != "" {
		return lv, nil
	}
	return c.getLatestVersionFromTags(ctx, logE, owner, repo)
}

// compare evaluates a tag against the current latest version.
// It attempts to parse the tag as semantic version and compares it.
// If parsing fails, it falls back to string comparison.
//
// Parameters:
//   - latestSemver: current latest semantic version
//   - latestVersion: current latest version string
//   - tag: new tag to compare
//
// Returns the updated latest semantic version, latest version string, and any error.
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
			return v, "", nil
		}
		return latestSemver, "", nil
	}
	return v, "", nil
}

// getLatestVersionFromReleases finds the latest version from repository releases.
// It retrieves releases from GitHub API and compares them to find the highest
// version using semantic versioning when possible, falling back to string comparison.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - logE: logrus entry for structured logging
//   - owner: repository owner
//   - repo: repository name
//   - currentVersion: current version to check if stable (empty string to include all versions)
//
// Returns the latest version string or an error.
func (c *Controller) getLatestVersionFromReleases(ctx context.Context, logE *logrus.Entry, owner string, repo string, currentVersion string) (string, error) {
	release, _, err := c.repositoriesService.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return c.listReleasesAndGetLatest(ctx, logE, owner, repo, currentVersion)
	}
	return release.GetTagName(), nil
}

func (c *Controller) listReleasesAndGetLatest(ctx context.Context, logE *logrus.Entry, owner string, repo string, currentVersion string) (string, error) {
	opts := &github.ListOptions{
		PerPage: 30, //nolint:mnd
	}
	releases, _, err := c.repositoriesService.ListReleases(ctx, owner, repo, opts)
	if err != nil {
		return "", fmt.Errorf("list releases: %w", err)
	}

	// Check if current version is stable (issue #1095)
	currentIsStable := false
	if currentVersion != "" {
		cv, err := version.NewVersion(currentVersion)
		if err == nil && cv.Prerelease() == "" {
			currentIsStable = true
		}
	}

	var latestSemver *version.Version
	latestVersion := ""
	for _, release := range releases {
		// Skip prereleases if current version is stable (issue #1095)
		if currentIsStable && release.GetPrerelease() {
			continue
		}
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

// getLatestVersionFromTags finds the latest version from repository tags.
// It retrieves tags from GitHub API and compares them to find the highest
// version using semantic versioning when possible, falling back to string comparison.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - logE: logrus entry for structured logging
//   - owner: repository owner
//   - repo: repository name
//
// Returns the latest version string or an error.
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

// review creates a pull request review comment.
// It constructs a comment with either a suggestion or error message and
// posts it to the specified pull request using the GitHub API.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - filePath: path to the file being reviewed
//   - sha: commit SHA for the review
//   - line: line number in the file
//   - suggestion: code suggestion text (mutually exclusive with err)
//   - err: error to report (mutually exclusive with suggestion)
//
// Returns the HTTP status code and any error.
func (c *Controller) review(ctx context.Context, filePath string, sha string, line int, suggestion string, err error) (int, error) {
	cmt := &github.PullRequestComment{
		Body: github.Ptr(""),
		Path: github.Ptr(filePath),
		Line: github.Ptr(line),
	}
	if sha != "" {
		cmt.CommitID = github.Ptr(sha)
	}
	const header = "Reviewed by [pinact](https://github.com/suzuki-shunsuke/pinact)"
	switch {
	case suggestion != "":
		cmt.Body = github.Ptr(fmt.Sprintf("%s\n```suggestion\n%s\n```", header, suggestion))
	case err != nil:
		cmt.Body = github.Ptr(fmt.Sprintf("%s\n%s", header, err.Error()))
	default:
		return 0, errors.New("either suggestion or error must be provided")
	}
	_, resp, e := c.pullRequestsService.CreateComment(ctx, c.param.Review.RepoOwner, c.param.Review.RepoName, c.param.Review.PullRequest, cmt)
	code := 0
	if resp != nil {
		code = resp.StatusCode
	}
	if e != nil {
		return code, fmt.Errorf("create a review comment: %w", e)
	}
	return code, nil
}
