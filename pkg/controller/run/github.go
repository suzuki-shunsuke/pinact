package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/hashicorp/go-version"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
)

type RepositoriesService interface {
	ListTags(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error)
	GetCommitSHA1(ctx context.Context, owner, repo, ref, lastSHA string) (string, *github.Response, error)
	ListReleases(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error)
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
	arr := make([]*github.RepositoryRelease, 0, len(releases))
	for _, r := range releases {
		// Ignore draft releases
		if r.GetDraft() {
			continue
		}
		arr = append(arr, r)
	}
	r.Releases[key] = &ListReleasesResult{
		Releases: arr,
		Response: resp,
		err:      err,
	}
	return arr, resp, err //nolint:wrapcheck
}

// getLatestVersion determines the latest version of a repository.
// It first tries to get the latest version from releases, and if that fails
// or returns empty, it falls back to getting the latest version from tags.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - logger: slog logger for structured logging
//   - owner: repository owner
//   - repo: repository name
//   - currentVersion: current version to check if stable (empty string to include all versions)
//
// Returns the latest version string or an error.
func (c *Controller) getLatestVersion(ctx context.Context, logger *slog.Logger, owner, repo, currentVersion string) (string, error) {
	isStable := isStableVersion(currentVersion)
	lv, err := c.getLatestVersionFromReleases(ctx, logger, owner, repo, isStable)
	if err != nil {
		slogerr.WithError(logger, err).Debug("get the latest version from releases")
	}
	if lv != "" {
		return lv, nil
	}
	return c.getLatestVersionFromTags(ctx, logger, owner, repo, isStable)
}

func isStableVersion(v string) bool {
	if v == "" {
		return false
	}
	cv, err := version.NewVersion(v)
	return err == nil && cv.Prerelease() == ""
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
//   - logger: slog logger for structured logging
//   - owner: repository owner
//   - repo: repository name
//   - currentVersion: current version to check if stable (empty string to include all versions)
//
// Returns the latest version string or an error.
func (c *Controller) getLatestVersionFromReleases(ctx context.Context, logger *slog.Logger, owner, repo string, isStable bool) (string, error) {
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
		// Skip prereleases if current version is stable (issue #1095)
		if isStable && release.GetPrerelease() {
			continue
		}
		tag := release.GetTagName()
		ls, lv, err := compare(latestSemver, latestVersion, tag)
		latestSemver = ls
		latestVersion = lv
		if err != nil {
			slogerr.WithError(logger, err).Debug("compare tags", "tag", tag)
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
// It filters out prerelease versions when currentVersion is stable.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - logger: slog logger for structured logging
//   - owner: repository owner
//   - repo: repository name
//   - currentVersion: current version to check if stable (empty string to include all versions)
//
// Returns the latest version string or an error.
func (c *Controller) getLatestVersionFromTags(ctx context.Context, logger *slog.Logger, owner, repo string, isStable bool) (string, error) {
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

		// Skip prereleases if current version is stable (issue #1095)
		if isStable {
			if tv, err := version.NewVersion(t); err == nil && tv.Prerelease() != "" {
				continue
			}
		}

		ls, lv, err := compare(latestSemver, latestVersion, t)
		latestSemver = ls
		latestVersion = lv
		if err != nil {
			slogerr.WithError(logger, err).Debug("compare tags", "tag", tag)
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
func (c *Controller) review(ctx context.Context, filePath, sha string, line int, suggestion string, err error) (int, error) {
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
