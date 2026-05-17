package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/github"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
)

var (
	usesPattern          = regexp.MustCompile(`^( *(?:- +)?['"]?uses['"]? *: +)(['"]?)(.*?)@([^ '"]+)['"]?(?:( +# +(?:tag=)?)(v?\d+[^ ]*)(.*))?`)
	fullCommitSHAPattern = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	semverPattern        = regexp.MustCompile(`^v?\d+\.\d+\.\d+[^ ]*$`)
	shortTagPattern      = regexp.MustCompile(`^v?\d+(\.\d+)?$`)
)

type Action struct {
	Uses                    string
	Name                    string
	Version                 string
	VersionComment          string
	VersionCommentSeparator string
	RepoOwner               string
	RepoName                string
	Quote                   string
	Suffix                  string
}

type VersionType int

const (
	Semver VersionType = iota
	Shortsemver
	FullCommitSHA
	Empty
	Other
)

// getVersionType determines the type of version string.
// It analyzes the version format to classify it as semantic version,
// short semantic version, full commit SHA, empty, or other.
func getVersionType(v string) VersionType {
	if v == "" {
		return Empty
	}
	if fullCommitSHAPattern.MatchString(v) {
		return FullCommitSHA
	}
	if semverPattern.MatchString(v) {
		return Semver
	}
	if shortTagPattern.MatchString(v) {
		return Shortsemver
	}
	return Other
}

// parseAction extracts action information from a YAML line.
// It uses regular expressions to parse 'uses' statements and extract
// action name, version, comments, and formatting details.
func parseAction(line string) *Action {
	matches := usesPattern.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}
	return &Action{
		Uses:                    matches[1], // " - uses: "
		Quote:                   matches[2], // empty, ', "
		Name:                    matches[3], // local action is excluded by the regular expression because local action doesn't have version @
		Version:                 matches[4], // full commit hash, main, v3, v3.0.0
		VersionCommentSeparator: matches[5], // empty, " # ", " # tag="
		VersionComment:          matches[6], // empty, v1, v3.0.0
		Suffix:                  matches[7],
	}
}

var ErrCantPinned = errors.New("action can't be pinned")

// ignoreAction checks if an action should be ignored based on configuration.
// It evaluates the action against all ignore rules in the configuration.
func (c *Controller) ignoreAction(action *Action) bool {
	for _, ignoreAction := range c.cfg.IgnoreActions {
		if ignoreAction.Match(action.Name, action.Version) {
			return true
		}
	}
	return false
}

// excludeAction checks if an action should be excluded based on exclude patterns.
// It tests the action name against all configured exclude regular expressions.
func (c *Controller) excludeAction(actionName string) bool {
	for _, exclude := range c.param.Excludes {
		if exclude.MatchString(actionName) {
			return true
		}
	}
	return false
}

// excludeByIncludes checks if an action should be excluded due to include patterns.
// When include patterns are specified, only actions matching include patterns
// are processed, so this returns true if the action doesn't match any include pattern.
func (c *Controller) excludeByIncludes(actionName string) bool {
	if len(c.param.Includes) == 0 {
		return false
	}
	for _, include := range c.param.Includes {
		if include.MatchString(actionName) {
			return false
		}
	}
	return true
}

// parseLine processes a single line from a workflow file.
// It parses the line for action usage, applies filtering rules, and determines
// what modifications (if any) should be made based on the operation mode.
//
// As of v4, parseLine also runs the passive -min-age check on the final pinned
// SHA. The check is skipped when -min-age is not set or no git service is
// available. On violation it returns the patched line together with ErrMinAge
// so the caller can apply the fix and bump the exit code to ExitCodeUnfixable.
func (c *Controller) parseLine(ctx context.Context, logger *slog.Logger, line string) (s string, e error) {
	attrs := slogerr.NewAttrs(2) //nolint:mnd
	defer func() {
		e = attrs.With(e)
	}()
	action := parseAction(line)
	if action == nil {
		logger.Debug("unmatch")
		return "", nil
	}

	logger = attrs.Add(logger, "action", action.Name+"@"+action.Version)

	if c.shouldSkipAction(logger, action) {
		return "", nil
	}

	if !c.parseActionName(action) {
		logger.Debug("ignore line")
		return "", nil
	}

	resolved, err := c.cfg.ResolveRules(&config.MatchInput{
		ActionName:         action.Name,
		ActionRepoOwner:    action.RepoOwner,
		ActionRepoName:     action.RepoName,
		ActionRepoFullName: action.RepoOwner + "/" + action.RepoName,
		ActionRef:          action.Version,
		VersionComment:     action.VersionComment,
	})
	if err != nil {
		return "", fmt.Errorf("resolve rules: %w", err)
	}
	if resolved.Ignore {
		logger.Debug("ignore the action by a rule")
		return "", nil
	}

	newLine, err := c.processAction(ctx, logger, action, attrs)
	if err != nil {
		return newLine, err
	}
	if sha := c.finalPinnedSHA(action, newLine); sha != "" {
		minAge := c.effectiveMinAge(resolved)
		if minAgeErr := c.checkSHAMinAge(ctx, logger, action.RepoOwner, action.RepoName, sha, minAge); minAgeErr != nil {
			return newLine, minAgeErr
		}
	}
	return newLine, nil
}

// effectiveMinAge resolves the min-age threshold for a single action using the
// precedence: CLI flag > rules > top-level config. A CLI value of 0 means the
// flag was unset, so the config fallback applies. A rule that explicitly sets
// min_age to 0 disables the check for the matched action.
func (c *Controller) effectiveMinAge(resolved *config.Resolved) int {
	if c.param.MinAge > 0 {
		return c.param.MinAge
	}
	if resolved.MinAge != nil {
		return *resolved.MinAge
	}
	return c.cfg.MinAge
}

// finalPinnedSHA returns the commit SHA that the action will resolve to after
// pinact runs, or "" if no SHA is involved (e.g., an unpinnable branch that
// will surface as ErrCantPinned elsewhere). If parseLine produced a patched
// line, the SHA is extracted from the new line; otherwise it falls back to
// action.Version when that is itself a full commit SHA.
func (c *Controller) finalPinnedSHA(action *Action, newLine string) string {
	if newLine != "" {
		if m := fullCommitSHAPattern.FindString(newLine); m != "" {
			return m
		}
	}
	if getVersionType(action.Version) == FullCommitSHA {
		return action.Version
	}
	return ""
}

// checkSHAMinAge looks up the commit date of sha and returns ErrMinAge if the
// commit is younger than the min-age cutoff. minAge is the effective threshold
// for this action, already merged across CLI flag, rules, and config defaults.
// Returns nil when minAge is 0 or negative, the git service is unavailable, or
// -no-api is set.
func (c *Controller) checkSHAMinAge(ctx context.Context, logger *slog.Logger, owner, repo, sha string, minAge int) error {
	if minAge <= 0 || c.gitService == nil || c.param.NoAPI {
		return nil
	}
	cutoff := c.param.Now.AddDate(0, 0, -minAge)
	commit, _, err := c.gitService.GetCommit(ctx, logger, owner, repo, sha)
	if err != nil {
		return fmt.Errorf("get commit for min-age check: %w", err)
	}
	committedAt := commit.GetCommitter().GetDate().Time
	if committedAt.After(cutoff) {
		logger.Warn(
			"min-age violation",
			"sha", sha,
			"committed_at", committedAt,
			"cutoff", cutoff,
		)
		return fmt.Errorf(
			"%w: %s/%s@%s committed at %s (cutoff %s)",
			ErrMinAge, owner, repo, sha,
			committedAt.Format("2006-01-02"),
			cutoff.Format("2006-01-02"),
		)
	}
	return nil
}

// shouldSkipAction checks if an action should be skipped based on filtering rules.
func (c *Controller) shouldSkipAction(logger *slog.Logger, action *Action) bool {
	if c.ignoreAction(action) {
		logger.Debug("ignore the action")
		return true
	}
	if c.excludeAction(action.Name) {
		logger.Debug("exclude the action")
		return true
	}
	if c.excludeByIncludes(action.Name) {
		logger.Debug("exclude the action")
		return true
	}
	return false
}

// processAction dispatches based on the action's version form. The version
// is the primary determinant (already-pinned SHA vs. semver tag vs. branch);
// the comment refines the behavior inside each branch.
//
// When -no-api is set, processAction short-circuits any GitHub API call:
// already-pinned SHAs are accepted as-is and everything else is reported as
// unfixable (ExitCodeUnfixable).
func (c *Controller) processAction(ctx context.Context, logger *slog.Logger, action *Action, attrs *slogerr.Attrs) (string, error) {
	if c.param.NoAPI {
		if getVersionType(action.Version) == FullCommitSHA {
			return "", nil
		}
		return "", ErrCantPinned
	}
	switch getVersionType(action.Version) {
	case FullCommitSHA:
		return c.processPinnedVersion(ctx, logger, action, attrs)
	case Semver, Shortsemver:
		return c.processTaggedVersion(ctx, logger, action)
	default:
		return c.processUnpinnedVersion(ctx, logger, action)
	}
}

// processPinnedVersion handles actions whose Version is already a full commit
// SHA. The comment determines whether to verify, expand a short tag, or
// update to a newer release.
func (c *Controller) processPinnedVersion(ctx context.Context, logger *slog.Logger, action *Action, attrs *slogerr.Attrs) (string, error) {
	switch getVersionType(action.VersionComment) {
	case Semver:
		// @<sha> # v1.0.0
		return c.processPinnedSemverComment(ctx, logger, action)
	case Shortsemver:
		// @<sha> # v1
		logger = attrs.Add(logger, "version_annotation", action.VersionComment)
		return c.processPinnedShortsemverComment(ctx, logger, action)
	default:
		// Empty (@<sha>) or Other (@<sha> # hoge): already pinned, leave alone.
		return "", nil
	}
}

// processPinnedSemverComment handles @<sha> # v1.0.0.
func (c *Controller) processPinnedSemverComment(ctx context.Context, logger *slog.Logger, action *Action) (string, error) {
	if !c.param.Update {
		return c.verifyIfNeeded(ctx, logger, action)
	}
	lv, err := c.getLatestVersion(ctx, logger, action.RepoOwner, action.RepoName, action.VersionComment)
	if err != nil {
		return "", fmt.Errorf("get the latest version: %w", err)
	}
	if action.VersionComment == lv {
		return c.verifyIfNeeded(ctx, logger, action)
	}
	if !compareVersion(action.VersionComment, lv) {
		warnSkipOlderVersion(logger, action.VersionComment, lv)
		return "", nil
	}
	return c.patchToLatestVersion(ctx, logger, action, lv)
}

// processPinnedShortsemverComment handles @<sha> # v1.
func (c *Controller) processPinnedShortsemverComment(ctx context.Context, logger *slog.Logger, action *Action) (string, error) {
	if c.param.Update {
		lv, err := c.getLatestVersion(ctx, logger, action.RepoOwner, action.RepoName, action.VersionComment)
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		return c.patchToLatestVersion(ctx, logger, action, lv)
	}
	// replace Shortsemver to Semver
	longVersion, err := c.getLongVersionFromSHA(ctx, logger, action, action.Version)
	if err != nil {
		return "", err
	}
	if longVersion == "" {
		logger.Debug("a long tag whose SHA is same as SHA of the version annotation isn't found")
		return "", nil
	}
	return c.patchLine(action, action.Version, longVersion), nil
}

// processTaggedVersion handles actions whose Version is a semver or short
// semver tag. These are unpinned and must be pinned to a commit SHA, or
// updated to the latest version when --update is set.
func (c *Controller) processTaggedVersion(ctx context.Context, logger *slog.Logger, action *Action) (string, error) {
	typ := getVersionType(action.Version)
	switch getVersionType(action.VersionComment) {
	case Empty:
		// @v1 or @v1.0.0
		if c.param.Update {
			return c.updateToLatestVersion(ctx, logger, action)
		}
		return c.pinCurrentVersion(ctx, logger, action, typ)
	case Semver:
		// @v1 # v1.0.0 or @v1.0.0 # v1.0.0
		if !c.param.Update {
			return c.pinCurrentVersion(ctx, logger, action, typ)
		}
		lv, err := c.getLatestVersion(ctx, logger, action.RepoOwner, action.RepoName, action.VersionComment)
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		if action.VersionComment != lv && !compareVersion(action.VersionComment, lv) {
			warnSkipOlderVersion(logger, action.VersionComment, lv)
			return "", nil
		}
		return c.patchToLatestVersion(ctx, logger, action, lv)
	default:
		// Shortsemver or Other comment on an unpinned tag: invalid combination.
		return "", ErrCantPinned
	}
}

// processUnpinnedVersion handles actions whose Version is neither a SHA nor
// a semver tag (typically a branch name like main).
func (c *Controller) processUnpinnedVersion(ctx context.Context, logger *slog.Logger, action *Action) (string, error) {
	switch getVersionType(action.VersionComment) {
	case Empty:
		if c.matchBranchToTag(action.Version) {
			return c.convertBranchToLatestTag(ctx, logger, action)
		}
		return "", ErrCantPinned
	case Semver:
		if !c.param.Update {
			return "", ErrCantPinned
		}
		lv, err := c.getLatestVersion(ctx, logger, action.RepoOwner, action.RepoName, action.VersionComment)
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		if action.VersionComment == lv {
			return "", ErrCantPinned
		}
		if !compareVersion(action.VersionComment, lv) {
			warnSkipOlderVersion(logger, action.VersionComment, lv)
			return "", nil
		}
		return c.patchToLatestVersion(ctx, logger, action, lv)
	default:
		return "", ErrCantPinned
	}
}

// patchToLatestVersion fetches the commit SHA of the latest version and
// rewrites the action line to pin against it.
func (c *Controller) patchToLatestVersion(ctx context.Context, logger *slog.Logger, action *Action, lv string) (string, error) {
	sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, logger, action.RepoOwner, action.RepoName, lv, "")
	if err != nil {
		return "", fmt.Errorf("get the latest version: %w", err)
	}
	return c.patchLine(action, sha, lv), nil
}

func warnSkipOlderVersion(logger *slog.Logger, currentVersion, newVersion string) {
	logger.Warn(
		"skip updating because the current version is newer than the new version",
		"current_version", currentVersion,
		"new_version", newVersion,
	)
}

// matchBranchToTag reports whether v matches any of the --branch-to-tag regexps.
func (c *Controller) matchBranchToTag(v string) bool {
	for _, re := range c.param.BranchToTags {
		if re.MatchString(v) {
			return true
		}
	}
	return false
}

// convertBranchToLatestTag resolves an action's non-semver version (e.g. a
// branch name) to the latest stable tag of the action's repository and pins
// the line to its commit SHA. Falls back to including pre-releases only when
// no stable tag exists.
func (c *Controller) convertBranchToLatestTag(ctx context.Context, logger *slog.Logger, action *Action) (string, error) {
	lv, err := c.getLatestVersionWithStable(ctx, logger, action.RepoOwner, action.RepoName, true)
	if err != nil {
		return "", fmt.Errorf("get the latest stable version: %w", err)
	}
	if lv == "" {
		lv, err = c.getLatestVersionWithStable(ctx, logger, action.RepoOwner, action.RepoName, false)
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		if lv == "" {
			return "", ErrCantPinned
		}
	}
	sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, logger, action.RepoOwner, action.RepoName, lv, "")
	if err != nil {
		return "", fmt.Errorf("get a reference: %w", err)
	}
	return c.patchLine(action, sha, lv), nil
}

// updateToLatestVersion updates an action to its latest version.
func (c *Controller) updateToLatestVersion(ctx context.Context, logger *slog.Logger, action *Action) (string, error) {
	lv, err := c.getLatestVersion(ctx, logger, action.RepoOwner, action.RepoName, action.Version)
	if err != nil {
		return "", fmt.Errorf("get the latest version: %w", err)
	}
	sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, logger, action.RepoOwner, action.RepoName, lv, "")
	if err != nil {
		return "", fmt.Errorf("get a reference: %w", err)
	}
	return c.patchLine(action, sha, lv), nil
}

// pinCurrentVersion pins the current version to a commit SHA.
func (c *Controller) pinCurrentVersion(ctx context.Context, logger *slog.Logger, action *Action, typ VersionType) (string, error) {
	// Get commit hash from tag
	// https://docs.github.com/en/rest/git/refs?apiVersion=2022-11-28#get-a-reference
	sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, logger, action.RepoOwner, action.RepoName, action.Version, "")
	if err != nil {
		return "", fmt.Errorf("get a reference: %w", err)
	}
	longVersion := action.Version
	if typ == Shortsemver {
		v, err := c.getLongVersionFromSHA(ctx, logger, action, sha)
		if err != nil {
			return "", err
		}
		if v != "" {
			longVersion = v
		}
	}
	return c.patchLine(action, sha, longVersion), nil
}

// compareVersion compares two version strings.
// It attempts semantic version comparison first, falling back to
// string comparison if semantic parsing fails.
func compareVersion(currentVersion, newVersion string) bool {
	cv, err := version.NewVersion(currentVersion)
	if err != nil {
		return newVersion > currentVersion
	}
	nv, err := version.NewVersion(newVersion)
	if err != nil {
		return newVersion > currentVersion
	}
	return nv.GreaterThan(cv)
}

// verifyIfNeeded verifies the commit hash if verification is enabled.
func (c *Controller) verifyIfNeeded(ctx context.Context, logger *slog.Logger, action *Action) (string, error) {
	if c.param.IsVerify {
		if err := c.verify(ctx, logger, action); err != nil {
			return "", fmt.Errorf("verify the version annotation: %w", err)
		}
	}
	return "", nil
}

// patchLine reconstructs a workflow line with updated version and tag.
// It combines the action information with new version and tag to create
// the updated line with proper formatting and comments.
func (c *Controller) patchLine(action *Action, version, tag string) string {
	sep := action.VersionCommentSeparator
	if sep == "" {
		sep = c.cfg.Separator
	}
	return action.Uses + action.Quote + action.Name + "@" + version + action.Quote + sep + tag + action.Suffix
}

// getLongVersionFromSHA finds the full semantic version tag for a commit SHA.
// It searches through repository tags to find a tag that points to the given
// commit and matches the version comment prefix.
func (c *Controller) getLongVersionFromSHA(ctx context.Context, logger *slog.Logger, action *Action, sha string) (string, error) {
	opts := &github.ListOptions{
		PerPage: 100, //nolint:mnd
	}
	// Get long tag from commit hash
	for range 10 {
		tags, resp, err := c.repositoriesService.ListTags(ctx, logger, action.RepoOwner, action.RepoName, opts)
		if err != nil {
			return "", fmt.Errorf("list tags: %w", err)
		}
		for _, tag := range tags {
			if sha != tag.GetCommit().GetSHA() {
				continue
			}
			tagName := tag.GetName()
			if action.VersionComment == "" {
				if action.Version == tagName {
					continue
				}
			} else {
				if action.VersionComment == tagName {
					continue
				}
			}
			if strings.HasPrefix(tagName, action.VersionComment) {
				return tagName, nil
			}
		}
		if resp.NextPage == 0 {
			return "", nil
		}
		opts.Page = resp.NextPage
	}
	return "", nil
}

// parseActionName extracts repository owner and name from action name.
// It parses the action name to extract the repository owner and name
// components, which are needed for GitHub API calls.
func (c *Controller) parseActionName(action *Action) bool {
	a := strings.Split(action.Name, "/")
	if len(a) == 1 {
		// If it fails to extract the repository owner and name, ignore the action.
		return false
	}
	action.RepoOwner = a[0]
	action.RepoName = a[1]
	return true
}

// verify checks that an action's version SHA matches its version comment.
// It validates that the commit SHA in the action version matches the
// commit SHA of the version specified in the comment.
func (c *Controller) verify(ctx context.Context, logger *slog.Logger, action *Action) error {
	sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, logger, action.RepoOwner, action.RepoName, action.VersionComment, "")
	if err != nil {
		return fmt.Errorf("get a commit hash: %w", err)
	}
	if action.Version == sha {
		return nil
	}
	return slogerr.With( //nolint:wrapcheck
		errors.New("action_version must be equal to commit_hash_of_version_annotation"),
		"action", action.Name,
		"action_version", action.Version,
		"version_annotation", action.VersionComment,
		"commit_hash_of_version_annotation", sha,
		"help_docs", "https://github.com/suzuki-shunsuke/pinact/blob/main/docs/codes/001.md",
	)
}
