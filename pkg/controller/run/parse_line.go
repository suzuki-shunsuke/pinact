package run

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
	"github.com/suzuki-shunsuke/pinact/pkg/github"
)

var (
	usesPattern          = regexp.MustCompile(`^( *(?:- )?['"]?uses['"]? *: +)(['"]?)(.*?)@([^ '"]+)['"]?(?:( +# +(?:tag=)?)(v?\d+[^ ]*)(.*))?`)
	fullCommitSHAPattern = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	semverPattern        = regexp.MustCompile(`^v?\d+\.\d+\.\d+[^ ]*$`)
	shortTagPattern      = regexp.MustCompile(`^v?\d+(\.\d+)?$`)
)

type Action struct {
	Uses                string
	Name                string
	Version             string
	Tag                 string
	VersionTagSeparator string
	RepoOwner           string
	RepoName            string
	Quote               string
	Suffix              string
}

type VersionType int

const (
	Semver VersionType = iota
	Shortsemver
	FullCommitSHA
	Empty
	Other
)

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

func parseAction(line string) *Action {
	matches := usesPattern.FindStringSubmatch(line)
	if matches == nil {
		return nil
	}
	return &Action{
		Uses:                matches[1], // " - uses: "
		Quote:               matches[2], // empty, ', "
		Name:                matches[3], // local action is excluded by the regular expression because local action doesn't have version @
		Version:             matches[4], // full commit hash, main, v3, v3.0.0
		VersionTagSeparator: matches[5], // empty, " # ", " # tag="
		Tag:                 matches[6], // empty, v1, v3.0.0
		Suffix:              matches[7],
	}
}

var ErrCantPinned = errors.New("action can't be pinned")

func (c *Controller) parseLine(ctx context.Context, logE *logrus.Entry, line string) (string, error) { //nolint:cyclop
	action := parseAction(line)
	if action == nil {
		// Ignore a line if the line doesn't use an action.
		logE.WithField("line", line).Debug("unmatch")
		return "", nil
	}

	logE = logE.WithField("action", action.Name)

	for _, ignoreAction := range c.cfg.IgnoreActions {
		if ignoreAction.Match(action.Name) {
			logE.WithFields(logrus.Fields{
				"line": line,
			}).Debug("ignore the action")
			return "", nil
		}
	}

	if c.cfg.Check {
		if fullCommitSHAPattern.MatchString(action.Version) {
			return "", nil
		}
		return "", logerr.WithFields(errors.New("action isn't pinned"), logrus.Fields{ //nolint:wrapcheck
			"action": action.Name + "@" + action.Version,
		})
	}

	if f := c.parseActionName(action); !f {
		logE.WithField("line", line).Debug("ignore line")
		return "", nil
	}

	switch getVersionType(action.Tag) {
	case Empty:
		return c.parseNoTagLine(ctx, logE, action)
	case Semver:
		// @xxx # v3.0.0
		return c.parseSemverTagLine(ctx, logE, action)
	case Shortsemver:
		// @xxx # v3
		// @<full commit hash> # v3
		return c.parseShortSemverTagLine(ctx, logE, action)
	default:
		if c.cfg.Fail {
			return "", ErrCantPinned
		}
		return "", nil
	}
}

func (c *Controller) parseNoTagLine(ctx context.Context, logE *logrus.Entry, action *Action) (string, error) { //nolint:cyclop
	typ := getVersionType(action.Version)
	switch typ {
	case Shortsemver, Semver:
	default:
		if c.cfg.Fail {
			return "", ErrCantPinned
		}
		return "", nil
	}
	// @xxx
	if c.update {
		// get the latest version
		lv, err := c.getLatestVersion(ctx, logE, action.RepoOwner, action.RepoName)
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, action.RepoOwner, action.RepoName, lv, "")
		if err != nil {
			return "", fmt.Errorf("get a reference: %w", err)
		}
		return patchLine(action, sha, lv), nil
	}

	// Get commit hash from tag
	// https://docs.github.com/en/rest/git/refs?apiVersion=2022-11-28#get-a-reference
	// > The :ref in the URL must be formatted as heads/<branch name> for branches and tags/<tag name> for tags. If the :ref doesn't match an existing ref, a 404 is returned.
	sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, action.RepoOwner, action.RepoName, action.Version, "")
	if err != nil {
		return "", fmt.Errorf("get a reference: %w", err)
	}
	longVersion := action.Version
	if typ == Shortsemver {
		v, err := c.getLongVersionFromSHA(ctx, action, sha)
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

func (c *Controller) parseSemverTagLine(ctx context.Context, logE *logrus.Entry, action *Action) (string, error) {
	// @xxx # v3.0.0
	if c.update {
		// get the latest version
		lv, err := c.getLatestVersion(ctx, logE, action.RepoOwner, action.RepoName)
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		if action.Tag != lv {
			sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, action.RepoOwner, action.RepoName, lv, "")
			if err != nil {
				return "", fmt.Errorf("get the latest version: %w", err)
			}
			return patchLine(action, sha, lv), nil
		}
	}
	// verify commit hash
	if !c.cfg.IsVerify {
		return "", nil
	}
	// @xxx # v3.0.0
	// @<full commit hash> # v3.0.0
	if FullCommitSHA != getVersionType(action.Version) {
		return "", nil
	}
	if err := c.verify(ctx, action); err != nil {
		return "", fmt.Errorf("verify the version annotation: %w", err)
	}
	return "", nil
}

func (c *Controller) parseShortSemverTagLine(ctx context.Context, logE *logrus.Entry, action *Action) (string, error) {
	// @xxx # v3
	// @<full commit hash> # v3
	if FullCommitSHA != getVersionType(action.Version) {
		return "", nil
	}
	if c.update {
		lv, err := c.getLatestVersion(ctx, logE, action.RepoOwner, action.RepoName)
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, action.RepoOwner, action.RepoName, lv, "")
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		return patchLine(action, sha, lv), nil
	}
	// replace Shortsemer to Semver
	longVersion, err := c.getLongVersionFromSHA(ctx, action, action.Version)
	if err != nil {
		return "", err
	}
	if longVersion == "" {
		return "", errors.New("failed to get a long tag")
	}
	return patchLine(action, action.Version, longVersion), nil
}

func patchLine(action *Action, version, tag string) string {
	sep := action.VersionTagSeparator
	if sep == "" {
		sep = " # "
	}
	return action.Uses + action.Quote + action.Name + "@" + version + action.Quote + sep + tag + action.Suffix
}

func (c *Controller) getLongVersionFromSHA(ctx context.Context, action *Action, sha string) (string, error) {
	opts := &github.ListOptions{
		PerPage: 100, //nolint:mnd
	}
	// Get long tag from commit hash
	for range 10 {
		tags, resp, err := c.repositoriesService.ListTags(ctx, action.RepoOwner, action.RepoName, opts)
		if err != nil {
			return "", fmt.Errorf("list tags: %w", err)
		}
		for _, tag := range tags {
			if sha != tag.GetCommit().GetSHA() {
				continue
			}
			tagName := tag.GetName()
			if action.Tag == "" {
				if action.Version == tagName {
					continue
				}
			} else {
				if action.Tag == tagName {
					continue
				}
			}
			if strings.HasPrefix(tagName, action.Tag) {
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

// parseActionName returns true if the action is a target.
// Otherwise, it returns false.
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

func (c *Controller) verify(ctx context.Context, action *Action) error {
	sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, action.RepoOwner, action.RepoName, action.Tag, "")
	if err != nil {
		return fmt.Errorf("get a commit hash: %w", err)
	}
	if action.Version == sha {
		return nil
	}
	return logerr.WithFields(errors.New("action_version must be equal to commit_hash_of_version_annotation"), logrus.Fields{ //nolint:wrapcheck
		"action":                            action.Name,
		"action_version":                    action.Version,
		"version_annotation":                action.Tag,
		"commit_hash_of_version_annotation": sha,
		"help_docs":                         "https://github.com/suzuki-shunsuke/pinact/blob/main/docs/codes/001.md",
	})
}
