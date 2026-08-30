package run

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/github"
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
func (c *Controller) getLatestVersion(ctx context.Context, logger *slog.Logger, owner, repo, currentVersion string, resolved *config.Resolved) (string, error) {
	return c.getLatestVersionWithStable(ctx, logger, owner, repo, isStableVersion(currentVersion), currentVersion, resolved)
}

// getLatestVersionWithStable is the same as getLatestVersion but takes an
// explicit isStable flag instead of inferring it from currentVersion. Used by
// branch-to-tag, which has no semver-shaped currentVersion to infer from.
//
// majorRef is the version string used to derive the current major when
// keep-major is in effect. When the caller does not have a semver-shaped
// reference (branch-to-tag), passing "" disables the major filter.
func (c *Controller) getLatestVersionWithStable(ctx context.Context, logger *slog.Logger, owner, repo string, isStable bool, majorRef string, resolved *config.Resolved) (string, error) {
	// Calculate cutoff once for min-age filtering. Honors per-rule overrides
	// via effectiveMinAge (CLI > rules > config).
	var cutoff time.Time
	if mAge := c.effectiveMinAge(resolved); mAge > 0 {
		cutoff = c.param.Now.AddDate(0, 0, -mAge)
	}

	// Resolve effective keep-major and parse the current major. If keep-major
	// is requested but majorRef cannot be parsed, fall back to no constraint
	// and warn so the user sees why.
	var currentMajor *int64
	if c.effectiveKeepMajor(resolved) {
		m, ok := parseMajor(majorRef)
		if ok {
			currentMajor = &m
		} else {
			logger.Warn("keep-major: cannot parse current version comment as semver; falling back to no major constraint",
				"version", majorRef)
		}
	}

	lv, versions, err := c.getLatestVersionFromReleases(ctx, logger, owner, repo, isStable, cutoff, currentMajor)
	if err != nil {
		slogerr.WithError(logger, err).Debug("get the latest version from releases")
	}
	if lv != "" {
		return lv, nil
	}
	return c.getLatestVersionFromTags(ctx, logger, owner, repo, isStable, cutoff, versions, currentMajor)
}

// parseMajor extracts the major version from a semver-shaped string. Returns
// the major and ok=true on success.
func parseMajor(v string) (int64, bool) {
	if v == "" {
		return 0, false
	}
	cv, err := version.NewVersion(v)
	if err != nil {
		return 0, false
	}
	segs := cv.Segments64()
	if len(segs) == 0 {
		return 0, false
	}
	return segs[0], true
}

// skipMajorMismatch reports whether tag should be skipped because its major
// version differs from currentMajor. When currentMajor is nil or the tag is
// not parseable as semver, the candidate passes through unchanged.
func skipMajorMismatch(logger *slog.Logger, currentMajor *int64, tag string) bool {
	if currentMajor == nil {
		return false
	}
	v, err := version.NewVersion(tag)
	if err != nil {
		return false
	}
	segs := v.Segments64()
	if len(segs) == 0 {
		return false
	}
	if segs[0] == *currentMajor {
		return false
	}
	logger.Info("skip release: major version mismatch",
		"tag", tag,
		"current_major", *currentMajor,
		"candidate_major", segs[0])
	return true
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
// When currentMajor is non-nil, releases whose major version differs are skipped.
func (c *Controller) getLatestVersionFromReleases(ctx context.Context, logger *slog.Logger, owner, repo string, isStable bool, cutoff time.Time, currentMajor *int64) (string, map[string]struct{}, error) {
	opts := &github.ListOptions{
		PerPage: 30, //nolint:mnd
	}
	releases, _, err := c.repositoriesService.ListReleases(ctx, logger, owner, repo, opts)
	if err != nil {
		return "", nil, fmt.Errorf("list releases: %w", err)
	}
	versions := make(map[string]struct{}, len(releases))

	var latestSemver *version.Version
	latestVersion := ""
	for _, release := range releases {
		// Skip prereleases if current version is stable (issue #1095)
		if isStable && release.GetPrerelease() {
			continue
		}
		tag := release.GetTagName()
		versions[tag] = struct{}{}
		// Skip releases with a major-version mismatch when keep-major is set
		if skipMajorMismatch(logger, currentMajor, tag) {
			continue
		}
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
		return latestSemver.Original(), versions, nil
	}
	return latestVersion, versions, nil
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
// When currentMajor is non-nil, tags whose major version differs are skipped.
func (c *Controller) getLatestVersionFromTags(ctx context.Context, logger *slog.Logger, owner, repo string, isStable bool, cutoff time.Time, releaseVersions map[string]struct{}, currentMajor *int64) (string, error) {
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
		if _, ok := releaseVersions[t]; ok {
			// Skip tags that are already released
			continue
		}

		// Skip prereleases if current version is stable (issue #1095)
		if isStable {
			if tv, err := version.NewVersion(t); err == nil && tv.Prerelease() != "" {
				continue
			}
		}

		// Skip tags with a major-version mismatch when keep-major is set
		if skipMajorMismatch(logger, currentMajor, t) {
			continue
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
