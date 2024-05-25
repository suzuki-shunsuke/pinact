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
	usesPattern          = regexp.MustCompile(`^ +(?:- )?uses: +(.*)@([^ ]+)(?: +# +(?:tag=)?(v?\d+[^ ]*))?`)
	fullCommitSHAPattern = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
	semverPattern        = regexp.MustCompile(`^v?\d+\.\d+\.\d+[^ ]*$`)
	shortTagPattern      = regexp.MustCompile(`^v\d+$`)
)

type Action struct {
	Name      string
	Version   string
	Tag       string
	RepoOwner string
	RepoName  string
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

func (c *Controller) parseLine(ctx context.Context, logE *logrus.Entry, line string, cfg *Config) (string, error) { //nolint:cyclop,funlen
	matches := usesPattern.FindStringSubmatch(line)
	if matches == nil {
		// Ignore a line if the line doesn't use an action.
		logE.WithField("line", line).Debug("unmatch")
		return line, nil
	}
	action := &Action{
		Name:    matches[1], // local action is excluded by the regular expression because local action doesn't have version @
		Version: matches[2], // full commit hash, main, v3, v3.0.0
		Tag:     matches[3], // empty, v1, v3.0.0
	}

	for _, ignoreAction := range cfg.IgnoreActions {
		if action.Name == ignoreAction.Name {
			logE.WithFields(logrus.Fields{
				"line":   line,
				"action": action.Name,
			}).Debug("ignore the action")
			return line, nil
		}
	}

	if f := c.parseActionName(action); !f {
		logE.WithField("line", line).Debug("ignore line")
		return line, nil
	}

	switch getVersionType(action.Tag) {
	case Empty:
		typ := getVersionType(action.Version)
		switch typ {
		case Shortsemver, Semver:
		default:
			return line, nil
		}
		// @xxx
		// Get commit hash from tag
		// https://docs.github.com/en/rest/git/refs?apiVersion=2022-11-28#get-a-reference
		// > The :ref in the URL must be formatted as heads/<branch name> for branches and tags/<tag name> for tags. If the :ref doesn't match an existing ref, a 404 is returned.
		sha, _, err := c.repositoriesService.GetCommitSHA1(ctx, action.RepoOwner, action.RepoName, action.Version, "")
		if err != nil {
			logerr.WithError(logE, err).Warn("get a reference")
			return line, nil
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
		return c.patchLine(line, action, sha, longVersion), nil
	case Semver:
		// verify commit hash
		if !cfg.IsVerify {
			return line, nil
		}
		// @xxx # v3.0.0
		// @<full commit hash> # v3.0.0
		if FullCommitSHA != getVersionType(action.Version) {
			return line, nil
		}
		if err := c.verify(ctx, action); err != nil {
			return "", fmt.Errorf("verify the version annotation: %w", err)
		}
		return line, nil
	case Shortsemver:
		// @xxx # v3
		// @<full commit hash> # v3
		if FullCommitSHA != getVersionType(action.Version) {
			return line, nil
		}
		// replace Shortsemer to Semver
		longVersion, err := c.getLongVersionFromSHA(ctx, action, action.Version)
		if err != nil {
			return "", err
		}
		if longVersion == "" {
			logE.Debug("failed to get a long tag")
			return line, nil
		}
		return c.patchLine(line, action, action.Version, longVersion), nil
	default:
		return line, nil
	}
}

func (c *Controller) patchLine(line string, action *Action, version, tag string) string {
	if action.Tag == "" {
		if version == tag {
			return line
		}
		return strings.Replace(line, "@"+action.Version, fmt.Sprintf("@%s # %s", version, tag), 1)
	}
	return strings.Replace(line, fmt.Sprintf("@%s # %s", action.Version, action.Tag), fmt.Sprintf("@%s # %s", action.Version, tag), 1)
}

func (c *Controller) getLongVersionFromSHA(ctx context.Context, action *Action, sha string) (string, error) {
	opts := &github.ListOptions{
		PerPage: 100, //nolint:gomnd
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
