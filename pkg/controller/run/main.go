package run

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
	"github.com/suzuki-shunsuke/pinact/pkg/github"
	"gopkg.in/yaml.v3"
)

type Controller struct {
	repositoriesService RepositoriesService
	fs                  afero.Fs
}

func New(ctx context.Context) *Controller {
	gh := github.New(ctx)
	return &Controller{
		repositoriesService: &RepositoriesServiceImpl{
			tags:                map[string]*ListTagsResult{},
			commits:             map[string]*GetCommitSHA1Result{},
			RepositoriesService: gh.Repositories,
		},
		fs: afero.NewOsFs(),
	}
}

func NewController(repoService RepositoriesService, fs afero.Fs) *Controller {
	return &Controller{
		repositoriesService: repoService,
		fs:                  fs,
	}
}

type ParamRun struct {
	WorkflowFilePaths []string
	ConfigFilePath    string
	PWD               string
}

func (ctrl *Controller) Run(ctx context.Context, logE *logrus.Entry, param *ParamRun) error {
	cfg := &Config{}
	if err := ctrl.readConfig(param.ConfigFilePath, cfg); err != nil {
		return err
	}
	workflowFilePaths, err := ctrl.searchFiles(logE, param.WorkflowFilePaths, cfg, param.PWD)
	if err != nil {
		return fmt.Errorf("search target files: %w", err)
	}
	for _, workflowFilePath := range workflowFilePaths {
		logE := logE.WithField("workflow_file", workflowFilePath)
		if err := ctrl.runWorkflow(ctx, logE, workflowFilePath, cfg); err != nil {
			logerr.WithError(logE, err).Warn("update a workflow")
		}
	}
	return nil
}

func (ctrl *Controller) searchFiles(logE *logrus.Entry, workflowFilePaths []string, cfg *Config, pwd string) ([]string, error) {
	if len(workflowFilePaths) != 0 {
		return workflowFilePaths, nil
	}
	if len(cfg.Files) > 0 {
		return ctrl.searchFilesByConfig(logE, cfg, pwd)
	}
	return listWorkflows()
}

func (ctrl *Controller) searchFilesByConfig(logE *logrus.Entry, cfg *Config, pwd string) ([]string, error) {
	patterns := make([]*regexp.Regexp, 0, len(cfg.Files))
	for _, file := range cfg.Files {
		if file.Pattern == "" {
			// ignore
			continue
		}
		p, err := regexp.Compile(file.Pattern)
		if err != nil {
			return nil, fmt.Errorf("parse files[].pattern as a regular expression: %w", err)
		}
		patterns = append(patterns, p)
	}

	files := []string{}
	if err := fs.WalkDir(afero.NewIOFS(ctrl.fs), pwd, func(p string, dirEntry fs.DirEntry, e error) error {
		if e != nil {
			return nil //nolint:nilerr
		}
		if dirEntry.IsDir() {
			// ignore directory
			return nil
		}
		filePath, err := filepath.Rel(pwd, p)
		if err != nil {
			logE.WithFields(logrus.Fields{
				"pwd":  pwd,
				"path": p,
			}).WithError(err).Debug("get a relative path")
			return nil
		}
		for _, pattern := range patterns {
			if pattern.MatchString(filePath) {
				files = append(files, filePath)
				break
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("search target files: %w", err)
	}

	return files, nil
}

func getConfigPath(fs afero.Fs) (string, error) {
	for _, path := range []string{".pinact.yaml", ".github/pinact.yaml"} {
		f, err := afero.Exists(fs, path)
		if err != nil {
			return "", fmt.Errorf("check if %s exists: %w", path, err)
		}
		if f {
			return path, nil
		}
	}
	return "", nil
}

func (ctrl *Controller) readConfig(configFilePath string, cfg *Config) error {
	var err error
	if configFilePath == "" {
		configFilePath, err = getConfigPath(ctrl.fs)
		if err != nil {
			return err
		}
		if configFilePath == "" {
			return nil
		}
	}
	f, err := ctrl.fs.Open(configFilePath)
	if err != nil {
		return fmt.Errorf("open a configuration file: %w", err)
	}
	defer f.Close()
	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return fmt.Errorf("decode a configuration file as YAML: %w", err)
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

func (ctrl *Controller) parseLine(ctx context.Context, logE *logrus.Entry, line string, cfg *Config) (string, error) { //nolint:cyclop,funlen
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
	for _, ignoreAction := range cfg.IgnoreActions {
		if action.Name == ignoreAction.Name {
			logE.WithFields(logrus.Fields{
				"line":   line,
				"action": action.Name,
			}).Debug("ignore the action")
			return line, nil
		}
	}
	if f := ctrl.parseAction(action); !f {
		logE.WithField("line", line).Debug("ignore line")
		return line, nil
	}
	if action.Tag == "" { //nolint:nestif
		// @xxx
		// Get commit hash from tag
		// https://docs.github.com/en/rest/git/refs?apiVersion=2022-11-28#get-a-reference
		// > The :ref in the URL must be formatted as heads/<branch name> for branches and tags/<tag name> for tags. If the :ref doesn't match an existing ref, a 404 is returned.
		sha, _, err := ctrl.repositoriesService.GetCommitSHA1(ctx, action.RepoOwner, action.RepoName, action.Version, "")
		if err != nil {
			logerr.WithError(logE, err).Warn("get a reference")
			return line, nil
		}
		longVersion := action.Version
		if shortTagPattern.MatchString(action.Version) {
			v, err := ctrl.getLongVersionFromSHA(ctx, action, sha)
			if err != nil {
				return "", err
			}
			if v != "" {
				longVersion = v
			}
		}
		// @yyy # longVersion
		return ctrl.patchLine(line, action, sha, longVersion), nil
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
		if version == tag {
			return line
		}
		return strings.Replace(line, "@"+action.Version, fmt.Sprintf("@%s # %s", version, tag), 1)
	}
	return strings.Replace(line, fmt.Sprintf("@%s # %s", action.Version, action.Tag), fmt.Sprintf("@%s # %s", action.Version, tag), 1)
}

func (ctrl *Controller) runWorkflow(ctx context.Context, logE *logrus.Entry, workflowFilePath string, cfg *Config) error {
	lines, err := ctrl.readWorkflow(workflowFilePath)
	if err != nil {
		return err
	}
	changed := false
	for i, line := range lines {
		l, err := ctrl.parseLine(ctx, logE, line, cfg)
		if err != nil {
			logerr.WithError(logE, err).Error("parse a line")
			continue
		}
		if line != l {
			changed = true
		}
		lines[i] = l
	}
	if !changed {
		return nil
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
	for range 10 {
		tags, _, err := ctrl.repositoriesService.ListTags(ctx, action.RepoOwner, action.RepoName, opts)
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
