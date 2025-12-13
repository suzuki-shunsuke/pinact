package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
)

var (
	usesPattern          = regexp.MustCompile(`^( *(?:- )?['"]?uses['"]? *: +)(['"]?)(.*?)@([^ '"]+)['"]?(?:( +# +(?:tag=)?)(v?\d+[^ ]*)(.*))?`)
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
func (c *Controller) ignoreAction(logger *slog.Logger, action *Action) bool {
	for _, ignoreAction := range c.cfg.IgnoreActions {
		f, err := ignoreAction.Match(action.Name, action.Version, c.cfg.Version)
		if err != nil {
			slogerr.WithError(logger, err).Warn("match the action")
			continue
		}
		if f {
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
func (c *Controller) parseLine(ctx context.Context, logger *slog.Logger, line string) (s string, e error) { //nolint:cyclop
	attrs := slogerr.NewAttrs(2) //nolint:mnd
	defer func() {
		e = attrs.With(e)
	}()
	action := parseAction(line)
	if action == nil {
		// Ignore a line if the line doesn't use an action.
		logger.Debug("unmatch")
		return "", nil
	}

	logger = attrs.Add(logger, "action", action.Name+"@"+action.Version)

	if c.ignoreAction(logger, action) {
		logger.Debug("ignore the action")
		return "", nil
	}
	if c.excludeAction(action.Name) {
		logger.Debug("exclude the action")
		return "", nil
	}
	if c.excludeByIncludes(action.Name) {
		logger.Debug("exclude the action")
		return "", nil
	}

	if c.param.Check && !c.param.Diff && !c.param.Fix {
		if fullCommitSHAPattern.MatchString(action.Version) {
			return "", nil
		}
		return "", ErrActionNotPinned
	}

	if f := c.parseActionName(action); !f {
		logger.Debug("ignore line")
		return "", nil
	}

	switch getVersionType(action.VersionComment) {
	case Empty:
		return c.parseNoTagLine(ctx, logger, action)
	case Semver:
		// @xxx # v3.0.0
		return c.parseSemverTagLine(ctx, logger, action)
	case Shortsemver:
		// @xxx # v3
		// @<full commit hash> # v3
		logger = attrs.Add(logger, "version_annotation", action.VersionComment)
		return c.parseShortSemverTagLine(ctx, logger, action)
	default:
		if getVersionType(action.Version) == FullCommitSHA {
			return "", nil
		}
		return "", ErrCantPinned
	}
}

// parseNoTagLine processes actions without version comments.
// It handles pinning actions that don't have version annotations,
// either by updating to latest version or converting tags to commit SHAs.
func (c *Controller) parseNoTagLine(ctx context.Context, logger *slog.Logger, action *Action) (string, error) { //nolint:cyclop
	typ := getVersionType(action.Version)
	switch typ {
	case Shortsemver, Semver:
	case FullCommitSHA:
		return "", nil
	default:
		return "", ErrCantPinned
	}
	// @xxx
	if c.param.Update {
		// get the latest version
		lv, err := c.getLatestVersion(ctx, logger, action.RepoOwner, action.RepoName, action.Version)
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, logger, action.RepoOwner, action.RepoName, lv, "")
		if err != nil {
			return "", fmt.Errorf("get a reference: %w", err)
		}
		return patchLine(action, sha, lv), nil
	}

	// Get commit hash from tag
	// https://docs.github.com/en/rest/git/refs?apiVersion=2022-11-28#get-a-reference
	// > The :ref in the URL must be formatted as heads/<branch name> for branches and tags/<tag name> for tags. If the :ref doesn't match an existing ref, a 404 is returned.
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
	// @yyy # longVersion
	return patchLine(action, sha, longVersion), nil
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

// parseSemverTagLine processes actions with semantic version comments.
// It handles updating semantic versions to latest and verifying that
// commit SHAs match their corresponding version tags.
func (c *Controller) parseSemverTagLine(ctx context.Context, logger *slog.Logger, action *Action) (string, error) {
	// @xxx # v3.0.0
	if c.param.Update { //nolint:nestif
		// get the latest version
		lv, err := c.getLatestVersion(ctx, logger, action.RepoOwner, action.RepoName, action.VersionComment)
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		if action.VersionComment == lv {
			return "", nil
		}
		if !compareVersion(action.VersionComment, lv) {
			logger.Warn("skip updating because the current version is newer than the new version",
				"current_version", action.VersionComment,
				"new_version", lv,
			)
			return "", nil
		}
		if action.VersionComment != lv {
			sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, logger, action.RepoOwner, action.RepoName, lv, "")
			if err != nil {
				return "", fmt.Errorf("get the latest version: %w", err)
			}
			return patchLine(action, sha, lv), nil
		}
	}
	// verify commit hash
	if !c.param.IsVerify {
		return "", nil
	}
	// @xxx # v3.0.0
	// @<full commit hash> # v3.0.0
	if FullCommitSHA != getVersionType(action.Version) {
		return "", nil
	}
	if err := c.verify(ctx, logger, action); err != nil {
		return "", fmt.Errorf("verify the version annotation: %w", err)
	}
	return "", nil
}

// parseShortSemverTagLine processes actions with short semantic version comments.
// It handles expanding short versions (like v3) to full versions (like v3.1.0)
// and updating to latest versions when requested.
func (c *Controller) parseShortSemverTagLine(ctx context.Context, logger *slog.Logger, action *Action) (string, error) {
	// @xxx # v3
	// @<full commit hash> # v3
	if FullCommitSHA != getVersionType(action.Version) {
		return "", ErrCantPinned
	}
	if c.param.Update {
		lv, err := c.getLatestVersion(ctx, logger, action.RepoOwner, action.RepoName, action.VersionComment)
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, logger, action.RepoOwner, action.RepoName, lv, "")
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		return patchLine(action, sha, lv), nil
	}
	// replace Shortsemer to Semver
	longVersion, err := c.getLongVersionFromSHA(ctx, logger, action, action.Version)
	if err != nil {
		return "", err
	}
	if longVersion == "" {
		logger.Debug("a long tag whose SHA is same as SHA of the version annotation isn't found")
		return "", nil
	}
	return patchLine(action, action.Version, longVersion), nil
}

// patchLine reconstructs a workflow line with updated version and tag.
// It combines the action information with new version and tag to create
// the updated line with proper formatting and comments.
func patchLine(action *Action, version, tag string) string {
	sep := action.VersionCommentSeparator
	if sep == "" {
		sep = " # "
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
	return slogerr.With(errors.New("action_version must be equal to commit_hash_of_version_annotation"), //nolint:wrapcheck
		"action", action.Name,
		"action_version", action.Version,
		"version_annotation", action.VersionComment,
		"commit_hash_of_version_annotation", sha,
		"help_docs", "https://github.com/suzuki-shunsuke/pinact/blob/main/docs/codes/001.md",
	)
}
