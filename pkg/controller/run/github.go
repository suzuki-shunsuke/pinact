package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
)

// RepositoriesService defines the interface for GitHub Repositories API operations
// used by the Controller.
type RepositoriesService interface {
	ListTags(ctx context.Context, logger *slog.Logger, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error)
	ListReleases(ctx context.Context, logger *slog.Logger, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error)
	GetCommitSHA1(ctx context.Context, logger *slog.Logger, owner, repo, ref, lastSHA string) (string, *github.Response, error)
}

// GitService defines the interface for GitHub Git API operations
// used by the Controller.
type GitService interface {
	GetCommit(ctx context.Context, logger *slog.Logger, owner, repo, sha string) (*github.Commit, *github.Response, error)
}

// getLatestVersion determines the latest version of a repository.
// It first tries to get the latest version from releases, and if that fails
// or returns empty, it falls back to getting the latest version from tags.
func (c *Controller) getLatestVersion(ctx context.Context, logger *slog.Logger, owner, repo, currentVersion string) (string, error) {
	isStable := isStableVersion(currentVersion)

	// Calculate cutoff once for min-age filtering
	var cutoff time.Time
	if c.param.MinAge > 0 {
		cutoff = c.param.Now.AddDate(0, 0, -c.param.MinAge)
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
	releases, _, err := c.repositoriesService.ListReleases(ctx, logger, owner, repo, opts)
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
func checkTagCooldown(ctx context.Context, logger *slog.Logger, gitService GitService, owner, repo, tagName, sha string, cutoff time.Time) bool {
	if cutoff.IsZero() || gitService == nil || sha == "" {
		return false
	}
	commit, _, err := gitService.GetCommit(ctx, logger, owner, repo, sha)
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
	tags, _, err := c.repositoriesService.ListTags(ctx, logger, owner, repo, opts)
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
