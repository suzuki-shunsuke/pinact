package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

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

type GitService interface {
	GetCommit(ctx context.Context, owner, repo, sha string) (*github.Commit, *github.Response, error)
}

type GitServiceImpl struct {
	defaultGitService GitService
	ghesGitService    GitService
	logger            *slog.Logger
	Commits           map[string]*GetCommitResult
}

type GetCommitResult struct {
	Commit   *github.Commit
	Response *github.Response
	err      error
}

func (g *GitServiceImpl) SetServices(defaultService, ghesService GitService, logger *slog.Logger) {
	g.defaultGitService = defaultService
	g.ghesGitService = ghesService
	g.logger = logger
}

// GetCommit retrieves a commit object with caching and GHES fallback.
func (g *GitServiceImpl) GetCommit(ctx context.Context, owner, repo, sha string) (*github.Commit, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/%s", owner, repo, sha)
	if result, ok := g.Commits[key]; ok {
		return result.Commit, result.Response, result.err
	}

	commit, resp, err := g.getCommit(ctx, owner, repo, sha)
	g.Commits[key] = &GetCommitResult{
		Commit:   commit,
		Response: resp,
		err:      err,
	}
	return commit, resp, err //nolint:wrapcheck
}

// getCommit calls the API with GHES fallback logic.
// If GHES is enabled, it first tries GHES and falls back to github.com on 404.
func (g *GitServiceImpl) getCommit(ctx context.Context, owner, repo, sha string) (*github.Commit, *github.Response, error) {
	if g.ghesGitService == nil {
		return g.defaultGitService.GetCommit(ctx, owner, repo, sha) //nolint:wrapcheck
	}

	commit, resp, err := g.ghesGitService.GetCommit(ctx, owner, repo, sha)
	if err == nil {
		return commit, resp, nil
	}
	// Fallback to github.com only on 404
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		g.logger.Debug("GHES returned 404, falling back to github.com", "owner", owner, "repo", repo, "sha", sha)
		return g.defaultGitService.GetCommit(ctx, owner, repo, sha) //nolint:wrapcheck
	}
	// For other errors (401, 403, 500, etc.), return the error without fallback
	return commit, resp, err //nolint:wrapcheck
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
	defaultRepoService RepositoriesService
	ghesRepoService    RepositoriesService
	logger             *slog.Logger
	Tags               map[string]*ListTagsResult
	Commits            map[string]*GetCommitSHA1Result
	Releases           map[string]*ListReleasesResult
}

func (r *RepositoriesServiceImpl) SetServices(defaultService, ghesService RepositoriesService, logger *slog.Logger) {
	r.defaultRepoService = defaultService
	r.ghesRepoService = ghesService
	r.logger = logger
}

// GetCommitSHA1 retrieves the commit SHA for a given reference with caching and GHES fallback.
func (r *RepositoriesServiceImpl) GetCommitSHA1(ctx context.Context, owner, repo, ref, lastSHA string) (string, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/%s", owner, repo, ref)
	if result, ok := r.Commits[key]; ok {
		return result.SHA, result.Response, result.err
	}

	sha, resp, err := r.getCommitSHA1(ctx, owner, repo, ref, lastSHA)
	r.Commits[key] = &GetCommitSHA1Result{
		SHA:      sha,
		Response: resp,
		err:      err,
	}
	return sha, resp, err //nolint:wrapcheck
}

// getCommitSHA1 calls the API with GHES fallback logic.
// If GHES is enabled, it first tries GHES and falls back to github.com on 404.
func (r *RepositoriesServiceImpl) getCommitSHA1(ctx context.Context, owner, repo, ref, lastSHA string) (string, *github.Response, error) {
	if r.ghesRepoService == nil {
		return r.defaultRepoService.GetCommitSHA1(ctx, owner, repo, ref, lastSHA) //nolint:wrapcheck
	}

	sha, resp, err := r.ghesRepoService.GetCommitSHA1(ctx, owner, repo, ref, lastSHA)
	if err == nil {
		return sha, resp, nil
	}
	// Fallback to github.com only on 404
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		r.logger.Debug("GHES returned 404, falling back to github.com", "owner", owner, "repo", repo, "ref", ref)
		return r.defaultRepoService.GetCommitSHA1(ctx, owner, repo, ref, lastSHA) //nolint:wrapcheck
	}
	// For other errors (401, 403, 500, etc.), return the error without fallback
	return sha, resp, err //nolint:wrapcheck
}

type GetCommitSHA1Result struct {
	SHA      string
	Response *github.Response
	err      error
}

// ListTags retrieves repository tags with caching and GHES fallback.
func (r *RepositoriesServiceImpl) ListTags(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/%v", owner, repo, opts.Page)
	if result, ok := r.Tags[key]; ok {
		return result.Tags, result.Response, result.err
	}

	tags, resp, err := r.listTags(ctx, owner, repo, opts)
	r.Tags[key] = &ListTagsResult{
		Tags:     tags,
		Response: resp,
		err:      err,
	}
	return tags, resp, err //nolint:wrapcheck
}

// listTags calls the API with GHES fallback logic.
// If GHES is enabled, it first tries GHES and falls back to github.com on 404.
func (r *RepositoriesServiceImpl) listTags(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error) {
	if r.ghesRepoService == nil {
		return r.defaultRepoService.ListTags(ctx, owner, repo, opts) //nolint:wrapcheck
	}

	tags, resp, err := r.ghesRepoService.ListTags(ctx, owner, repo, opts)
	if err == nil {
		return tags, resp, nil
	}
	// Fallback to github.com only on 404
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		r.logger.Debug("GHES returned 404, falling back to github.com", "owner", owner, "repo", repo)
		return r.defaultRepoService.ListTags(ctx, owner, repo, opts) //nolint:wrapcheck
	}
	// For other errors (401, 403, 500, etc.), return the error without fallback
	return tags, resp, err //nolint:wrapcheck
}

// ListReleases retrieves repository releases with caching and GHES fallback.
func (r *RepositoriesServiceImpl) ListReleases(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error) {
	key := fmt.Sprintf("%s/%s/%v", owner, repo, opts.Page)
	if result, ok := r.Releases[key]; ok {
		return result.Releases, result.Response, result.err
	}

	releases, resp, err := r.listReleases(ctx, owner, repo, opts)
	arr := filterDraftReleases(releases)
	r.Releases[key] = &ListReleasesResult{
		Releases: arr,
		Response: resp,
		err:      err,
	}
	return arr, resp, err //nolint:wrapcheck
}

// listReleases calls the API with GHES fallback logic.
// If GHES is enabled, it first tries GHES and falls back to github.com on 404.
func (r *RepositoriesServiceImpl) listReleases(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error) {
	if r.ghesRepoService == nil {
		return r.defaultRepoService.ListReleases(ctx, owner, repo, opts) //nolint:wrapcheck
	}

	releases, resp, err := r.ghesRepoService.ListReleases(ctx, owner, repo, opts)
	if err == nil {
		return releases, resp, nil
	}
	// Fallback to github.com only on 404
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		r.logger.Debug("GHES returned 404, falling back to github.com", "owner", owner, "repo", repo)
		return r.defaultRepoService.ListReleases(ctx, owner, repo, opts) //nolint:wrapcheck
	}
	// For other errors (401, 403, 500, etc.), return the error without fallback
	return releases, resp, err //nolint:wrapcheck
}

func filterDraftReleases(releases []*github.RepositoryRelease) []*github.RepositoryRelease {
	arr := make([]*github.RepositoryRelease, 0, len(releases))
	for _, release := range releases {
		// Ignore draft releases
		if release.GetDraft() {
			continue
		}
		arr = append(arr, release)
	}
	return arr
}

type PullRequestsServiceImpl struct {
	defaultPRService PullRequestsService
	ghesPRService    PullRequestsService
}

func (p *PullRequestsServiceImpl) SetServices(defaultService, ghesService PullRequestsService) {
	p.defaultPRService = defaultService
	p.ghesPRService = ghesService
}

// CreateComment creates a pull request comment.
// If GHES is enabled, it always uses GHES (no fallback).
func (p *PullRequestsServiceImpl) CreateComment(ctx context.Context, owner, repo string, number int, comment *github.PullRequestComment) (*github.PullRequestComment, *github.Response, error) {
	if p.ghesPRService != nil {
		return p.ghesPRService.CreateComment(ctx, owner, repo, number, comment) //nolint:wrapcheck
	}
	return p.defaultPRService.CreateComment(ctx, owner, repo, number, comment) //nolint:wrapcheck
}

// getLatestVersion determines the latest version of a repository.
// It first tries to get the latest version from releases, and if that fails
// or returns empty, it falls back to getting the latest version from tags.
func (c *Controller) getLatestVersion(ctx context.Context, logger *slog.Logger, owner, repo, currentVersion string) (string, error) {
	isStable := isStableVersion(currentVersion)

	// Calculate cutoff once for min-age filtering
	var cutoff time.Time
	if c.param.MinAge > 0 {
		cutoff = time.Now().AddDate(0, 0, -c.param.MinAge)
	}

	lv, err := c.getLatestVersionFromReleases(ctx, logger, owner, repo, isStable, cutoff)
	if err != nil {
		slogerr.WithError(logger, err).Debug("get the latest version from releases")
	}
	if lv != "" {
		return lv, nil
	}
	return c.getLatestVersionFromTags(ctx, logger, owner, repo, isStable, cutoff)
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
func (c *Controller) getLatestVersionFromReleases(ctx context.Context, logger *slog.Logger, owner, repo string, isStable bool, cutoff time.Time) (string, error) {
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
		// Skip releases within cooldown period
		if !cutoff.IsZero() && release.GetPublishedAt().After(cutoff) {
			logger.Info("skip release due to cooldown",
				"tag", tag,
				"published_at", release.GetPublishedAt())
			continue
		}
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

// checkTagCooldown checks if a tag should be skipped due to cooldown period.
// It returns true if the tag should be skipped.
func checkTagCooldown(ctx context.Context, logger *slog.Logger, gitService *GitServiceImpl, owner, repo, tagName, sha string, cutoff time.Time) bool {
	if cutoff.IsZero() || gitService == nil || sha == "" {
		return false
	}
	commit, _, err := gitService.GetCommit(ctx, owner, repo, sha)
	if err != nil {
		slogerr.WithError(logger, err).Warn("skip tag: failed to get commit for cooldown check", "tag", tagName, "sha", sha)
		return true
	}
	if commit.GetCommitter().GetDate().After(cutoff) {
		logger.Info("skip tag due to cooldown",
			"tag", tagName,
			"committed_at", commit.GetCommitter().GetDate())
		return true
	}
	return false
}

// getLatestVersionFromTags finds the latest version from repository tags.
// It retrieves tags from GitHub API and compares them to find the highest
// version using semantic versioning when possible, falling back to string comparison.
// It filters out prerelease versions when currentVersion is stable.
func (c *Controller) getLatestVersionFromTags(ctx context.Context, logger *slog.Logger, owner, repo string, isStable bool, cutoff time.Time) (string, error) {
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

		// Skip tags within cooldown period
		if checkTagCooldown(ctx, logger, c.gitService, owner, repo, t, tag.GetCommit().GetSHA(), cutoff) {
			continue
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
