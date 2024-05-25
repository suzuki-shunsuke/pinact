package run

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
	"github.com/suzuki-shunsuke/pinact/pkg/github"
)

var usesPattern = regexp.MustCompile(`^ +(?:- )?uses: +(.*)@([^ ]+)(?: +# +(?:tag=)?(v\d+[^ ]*))?`)

type Action struct {
	Name      string
	Version   string
	Tag       string
	RepoOwner string
	RepoName  string
}

func (c *Controller) parseLine(ctx context.Context, logE *logrus.Entry, line string, cfg *Config) (string, error) { //nolint:cyclop,funlen
	matches := usesPattern.FindStringSubmatch(line)
	if matches == nil {
		// Ignore a line if the line doesn't use an action.
		logE.WithField("line", line).Debug("unmatch")
		return line, nil
	}
	action := &Action{
		Name:    matches[1],
		Version: matches[2], // full commit hash, main, v3, v3.0.0
		Tag:     matches[3], // empty, v1, v3.0.0, hoge
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

	if f := c.parseAction(action); !f {
		logE.WithField("line", line).Debug("ignore line")
		return line, nil
	}
	if action.Tag == "" { //nolint:nestif
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
		if shortTagPattern.MatchString(action.Version) {
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
	}
	// @xxx # v3
	// list releases
	// extract releases by commit hash
	if !shortTagPattern.MatchString(action.Tag) {
		logE.WithField("action_version", action.Version).Debug("ignore the line because the tag is not short")
		return line, nil
	}
	longVersion, err := c.getLongVersionFromSHA(ctx, action, action.Version)
	if err != nil {
		return "", err
	}
	if longVersion == "" {
		logE.Debug("failed to get a long tag")
		return line, nil
	}
	return c.patchLine(line, action, action.Version, longVersion), nil
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
		tags, _, err := c.repositoriesService.ListTags(ctx, action.RepoOwner, action.RepoName, opts)
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
		if len(tags) < opts.PerPage {
			return "", nil
		}
		opts.Page++
	}
	return "", nil
}

var shortTagPattern = regexp.MustCompile(`^v\d+$`)

// parseAction returns true if the action is a target.
// Otherwise, it returns false.
func (c *Controller) parseAction(action *Action) bool {
	a := strings.Split(action.Name, "/")
	if len(a) == 1 {
		// If it fails to extract the repository owner and name, ignore the action.
		return false
	}
	action.RepoOwner = a[0]
	action.RepoName = a[1]
	if action.Tag != "" && !shortTagPattern.MatchString(action.Tag) {
		// Ignore if the tag is not a short tag.
		// e.g. uses: actions/checkout@xxx # v2.0.0
		return false
	}
	return true
}
