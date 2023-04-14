package run

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
	"github.com/suzuki-shunsuke/pinact/pkg/github"
)

type Controller struct {
	GitService          GitService
	RepositoriesService RepositoriesService
}

func New(ctx context.Context) *Controller {
	gh := github.New(ctx)
	return &Controller{
		GitService: &GitServiceImpl{
			m:          map[string]*GetRefResponse{},
			GitService: gh.Git,
		},
		RepositoriesService: &RepositoriesServiceImpl{
			m:                   map[string]*ListTagsResult{},
			RepositoriesService: gh.Repositories,
		},
	}
}

func (ctrl *Controller) Run(ctx context.Context, logE *logrus.Entry, workflowFilePaths []string) error {
	if len(workflowFilePaths) == 0 {
		paths, err := listWorkflows()
		if err != nil {
			return err
		}
		workflowFilePaths = paths
	}
	for _, workflowFilePath := range workflowFilePaths {
		logE := logE.WithField("workflow_file", workflowFilePath)
		if err := ctrl.runWorkflow(ctx, logE, workflowFilePath); err != nil {
			logerr.WithError(logE, err).Warn("update a workflow")
		}
	}
	return nil
}

var usesPattern = regexp.MustCompile(`^ +(?:- )?uses: +(.*)@([^ ]+)(?: +# +(?:tag=)?(v\d+[^ ]*))?`)

type Action struct {
	Name      string
	Version   string
	Tag       string
	RepoOwner string
	RepoName  string
}

func (ctrl *Controller) parseLine(ctx context.Context, logE *logrus.Entry, line string) (string, error) { //nolint:cyclop
	matches := usesPattern.FindStringSubmatch(line)
	if matches == nil {
		logE.WithField("line", line).Debug("unmatch")
		return line, nil
	}
	action := &Action{
		Name:    matches[1],
		Version: matches[2],
		Tag:     matches[3],
	}
	if f := ctrl.parseAction(action); !f {
		logE.WithField("line", line).Debug("ignore line")
		return line, nil
	}
	if action.Tag == "" { //nolint:nestif
		// @xxx
		// Get commit hash from tag
		ref, _, err := ctrl.GitService.GetRef(ctx, action.RepoOwner, action.RepoName, action.Version)
		if err != nil {
			return "", fmt.Errorf("get a reference: %w", err)
		}
		longVersion := action.Version
		if shortTagPattern.MatchString(action.Version) {
			v, err := ctrl.getLongVersionFromSHA(ctx, action, ref.GetObject().GetSHA())
			if err != nil {
				return "", err
			}
			if v != "" {
				longVersion = v
			}
		}
		// @yyy # longVersion
		return ctrl.patchLine(line, action, ref.GetObject().GetSHA(), longVersion), nil
	}
	// @xxx # v3
	// list releases
	// extract releases by commit hash
	if !shortTagPattern.MatchString(action.Tag) {
		logE.WithField("action_version", action.Version).Debug("ignore the line because the tag is not short")
		return line, nil
	}
	longVersion, err := ctrl.getLongVersionFromSHA(ctx, action, action.Version)
	if err != nil {
		return "", err
	}
	if longVersion == "" {
		logE.Debug("failed to get a long tag")
		return line, nil
	}
	return ctrl.patchLine(line, action, action.Version, longVersion), nil
}

func (ctrl *Controller) patchLine(line string, action *Action, version, tag string) string {
	if action.Tag == "" {
		return strings.Replace(line, fmt.Sprintf("@%s", action.Version), fmt.Sprintf("@%s # %s", version, tag), 1)
	}
	return strings.Replace(line, fmt.Sprintf("@%s # %s", action.Version, action.Tag), fmt.Sprintf("@%s # %s", action.Version, tag), 1)
}

func (ctrl *Controller) runWorkflow(ctx context.Context, logE *logrus.Entry, workflowFilePath string) error {
	lines, err := ctrl.readWorkflow(workflowFilePath)
	if err != nil {
		return err
	}
	for i, line := range lines {
		l, err := ctrl.parseLine(ctx, logE, line)
		if err != nil {
			logerr.WithError(logE, err).Error("parse a line")
			continue
		}
		lines[i] = l
	}
	f, err := os.Create(workflowFilePath)
	if err != nil {
		return fmt.Errorf("create a workflow file: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		return fmt.Errorf("write a workflow file: %w", err)
	}
	return nil
}

func (ctrl *Controller) getLongVersionFromSHA(ctx context.Context, action *Action, sha string) (string, error) {
	opts := &github.ListOptions{
		PerPage: 100, //nolint:gomnd
	}
	// Get long tag from commit hash
	for i := 0; i < 10; i++ {
		tags, _, err := ctrl.RepositoriesService.ListTags(ctx, action.RepoOwner, action.RepoName, opts)
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

var shortTagPattern = regexp.MustCompile(`v\d+`)

func (ctrl *Controller) parseAction(action *Action) bool {
	a := strings.Split(action.Name, "/")
	if len(a) == 1 {
		return false
	}
	action.RepoOwner = a[0]
	action.RepoName = a[1]
	if action.Tag != "" && !shortTagPattern.MatchString(action.Tag) {
		return false
	}
	return true
}

func (ctrl *Controller) readWorkflow(workflowFilePath string) ([]string, error) {
	workflowReadFile, err := os.Open(workflowFilePath)
	if err != nil {
		return nil, fmt.Errorf("open a workflow file: %w", err)
	}
	defer workflowReadFile.Close()
	scanner := bufio.NewScanner(workflowReadFile)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan a workflow file: %w", err)
	}
	return lines, nil
}
