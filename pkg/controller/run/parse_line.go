package run

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
)

var (
	usesPattern          = regexp.MustCompile(`^( *(?:- )?['"]?uses['"]? *: +)(['"]?)(.*?)@([^ '"]+)['"]?(?:( +# +(?:tag=)?)(v?\d+(\.\d+){0,2}[^ ]*)(.*))?`)
	fullCommitSHAPattern = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	versionTagPattern    = regexp.MustCompile(`^v?\d+(\.\d+){0,2}[^ ]*$`)
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
	VersionTag VersionType = iota
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
	if versionTagPattern.MatchString(v) {
		return VersionTag
	}
	return Other
}

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

//nolint:cyclop
func (c *Controller) parseLine(ctx context.Context, logE *logrus.Entry, line string) (s string, e error) {
	defer func() {
		e = logerr.WithFields(e, logE.Data)
	}()
	action := parseAction(line)
	if action == nil {
		// Ignore a line if the line doesn't use an action.
		logE.WithField("line", line).Debug("unmatch")
		return "", nil
	}

	logE = logE.WithField("action", action.Name+"@"+action.Version)

	for _, ignoreAction := range c.cfg.IgnoreActions {
		f, err := ignoreAction.Match(action.Name, action.Version, c.cfg.Version)
		if err != nil {
			logerr.WithError(logE, err).Warn("match the action")
			continue
		}
		if f {
			logE.Debug("ignore the action")
			return "", nil
		}
	}

	if c.param.Check {
		if fullCommitSHAPattern.MatchString(action.Version) {
			return "", nil
		}
		return "", ErrNotPinned
	}

	if f := c.parseActionName(action); !f {
		logE.WithField("line", line).Debug("ignore line")
		return "", nil
	}

	switch getVersionType(action.VersionComment) {
	case Empty:
		return c.parseNoTagLine(ctx, logE, action)
	case VersionTag:
		// @xxx # v3
		// @xxx # v3.0.0
		// @<full commit hash> # v3
		logE = logE.WithField("version_annotation", action.VersionComment)
		return c.parseTagLine(ctx, logE, action)
	default:
		if getVersionType(action.Version) == FullCommitSHA {
			return "", nil
		}
		return "", ErrCantPinned
	}
}

//nolint:cyclop
func (c *Controller) parseNoTagLine(ctx context.Context, logE *logrus.Entry, action *Action) (string, error) {
	typ := getVersionType(action.Version)
	switch typ {
	case VersionTag:
	case FullCommitSHA:
		return "", nil
	default:
		return "", ErrCantPinned
	}
	// @xxx
	if c.param.Update {
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

	version := action.Version
	if typ == VersionTag {
		v, err := c.getTagFromSHA(ctx, action, sha)
		if err != nil {
			return "", err
		}
		if v != "" {
			version = v
		}
	}

	return patchLine(action, sha, version), nil
}

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

//nolint:cyclop,nestif
func (c *Controller) parseTagLine(ctx context.Context, logE *logrus.Entry, action *Action) (string, error) {
	if c.param.Update {
		// get the latest version
		lv, err := c.getLatestVersion(ctx, logE, action.RepoOwner, action.RepoName)
		if err != nil {
			return "", fmt.Errorf("get the latest version: %w", err)
		}
		if action.VersionComment == lv {
			return "", nil
		}
		if !compareVersion(action.VersionComment, lv) {
			logE.WithFields(logrus.Fields{
				"current_version": action.VersionComment,
				"new_version":     lv,
			}).Warn("skip updating because the current version is newer than the new version")
			return "", nil
		}
		if action.VersionComment != lv {
			sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, action.RepoOwner, action.RepoName, lv, "")
			if err != nil {
				return "", fmt.Errorf("get the latest version: %w", err)
			}
			return patchLine(action, sha, lv), nil
		}
	}

	// ensure the commit sha is valid
	if FullCommitSHA != getVersionType(action.Version) {
		return "", ErrCantPinned
	}

	// versionParts is a slice of strings, each containing a group of
	// consecutive numbers in the version comment, for example:
	//     3.0.0   => []string{"3","0","0"}
	//     1.23.44 => []string{"1","23","44"}
	versionParts := regexp.MustCompile(`\d+`).FindAllString(action.VersionComment, -1)

	// the number of parts that comprise a semantic version (MAJOR.MINOR.PATCH)
	// ... this exists to avoid the error generated by go-mnd when using the int
	// directly in the comparison below
	semanticVersionParts := 3

	// if we have a "short version" (e.g. not a complete `1.2.3` semver), check
	// for a tag that matches the commit hash. note that this could be the same
	// tag that's specified.
	//
	// using != supports validation against version tags like:
	// - v1 (1 part)
	// - v1.2 (2 parts)
	// - v1.2.3.4 (4 parts)
	if len(versionParts) != semanticVersionParts {
		tag, err := c.getTagFromSHA(ctx, action, action.Version)
		if err != nil {
			return "", err
		}
		if tag == "" {
			logE.Warn("unable to find a tag with the specified SHA")
			tag = action.VersionComment
		}
		return patchLine(action, action.Version, tag), nil
	}

	if !c.param.IsVerify {
		return "", nil
	}

	if err := c.verify(ctx, action); err != nil {
		return "", fmt.Errorf("verify the version annotation: %w", err)
	}

	return "", nil
}

func patchLine(action *Action, version, tag string) string {
	sep := action.VersionCommentSeparator
	if sep == "" {
		sep = " # "
	}
	return action.Uses + action.Quote + action.Name + "@" + version + action.Quote + sep + tag + action.Suffix
}

func (c *Controller) getTagFromSHA(ctx context.Context, action *Action, sha string) (string, error) {
	opts := &github.ListOptions{
		PerPage: 100, //nolint:mnd
	}

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
	sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, action.RepoOwner, action.RepoName, action.VersionComment, "")
	if err != nil {
		return fmt.Errorf("get a commit hash: %w", err)
	}
	if action.Version == sha {
		return nil
	}
	return logerr.WithFields(errors.New("action_version must be equal to commit_hash_of_version_annotation"), logrus.Fields{ //nolint:wrapcheck
		"action":                            action.Name,
		"action_version":                    action.Version,
		"version_annotation":                action.VersionComment,
		"commit_hash_of_version_annotation": sha,
		"help_docs":                         "https://github.com/suzuki-shunsuke/pinact/blob/main/docs/codes/001.md",
	})
}
