package run

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
)

type ParamRun struct {
	WorkflowFilePaths []string
	ConfigFilePath    string
	PWD               string
	IsVerify          bool
	Update            bool
	Check             bool
	IsGitHubActions   bool
	Fail              bool
	Stderr            io.Writer
	Review            *Review
}

type Review struct {
	RepoOwner   string
	RepoName    string
	PullRequest int
	SHA         string
}

func (r *Review) Valid() bool {
	return r != nil && r.RepoOwner != "" && r.RepoName != "" && r.PullRequest > 0
}

func (c *Controller) Run(ctx context.Context, logE *logrus.Entry) error {
	if err := c.readConfig(); err != nil {
		return err
	}
	workflowFilePaths, err := c.searchFiles()
	if err != nil {
		return fmt.Errorf("search target files: %w", err)
	}

	failed := false
	for _, workflowFilePath := range workflowFilePaths {
		logE := logE.WithField("workflow_file", workflowFilePath)
		if err := c.runWorkflow(ctx, logE, workflowFilePath); err != nil {
			if c.param.Check {
				failed = true
				if !errors.Is(err, ErrActionsNotPinned) {
					logerr.WithError(logE, err).Error("check a workflow")
				}
				continue
			}
			failed = true
			if errors.Is(err, ErrActionsNotPinned) {
				continue
			}
			logerr.WithError(logE, err).Error("update a workflow")
		}
	}
	if failed {
		return ErrActionsNotPinned
	}
	return nil
}

func (c *Controller) readConfig() error {
	p, err := c.cfgFinder.Find(c.param.ConfigFilePath)
	if err != nil {
		return fmt.Errorf("find a configurationfile: %w", err)
	}
	c.param.ConfigFilePath = p
	cfg := &config.Config{}
	if err := c.cfgReader.Read(cfg, c.param.ConfigFilePath); err != nil {
		return fmt.Errorf("read a config file: %w", err)
	}
	c.cfg = cfg
	return nil
}

var (
	ErrActionsNotPinned = errors.New("action aren't pinned")
	ErrActionNotPinned  = errors.New("action isn't pinned")
)

func (c *Controller) runWorkflow(ctx context.Context, logE *logrus.Entry, workflowFilePath string) error { //nolint:cyclop,gocognit,funlen
	lines, err := c.readWorkflow(workflowFilePath)
	if err != nil {
		return err
	}
	changed := false
	failed := false
	for i, line := range lines {
		logE := logE.WithFields(logrus.Fields{
			"line_number": i + 1,
			"line":        line,
		})
		l, err := c.parseLine(ctx, logE, line)
		if err != nil { //nolint:nestif
			failed = true
			logerr.WithError(logE, err).Error("parse a line")
			if c.param.Review != nil {
				if err := c.review(ctx, workflowFilePath, c.param.Review.SHA, i+1, "", err); err != nil {
					logerr.WithError(logE, err).Error("create a review comment")
					if c.param.IsGitHubActions {
						fmt.Fprintf(c.param.Stderr, "::error file=%s,line=%d,title=pinact error::%s\n", workflowFilePath, i+1, err)
					}
				}
			} else {
				if c.param.IsGitHubActions {
					fmt.Fprintf(c.param.Stderr, "::error file=%s,line=%d,title=pinact error::%s\n", workflowFilePath, i+1, err)
				}
			}
			continue
		}
		if l == "" || line == l {
			continue
		}
		logE = logE.WithField("new_line", l)
		changed = true
		lines[i] = l
		if c.param.Review != nil {
			if err := c.review(ctx, workflowFilePath, c.param.Review.SHA, i+1, l, nil); err != nil {
				logerr.WithError(logE, err).Error("create a review comment")
			}
		}
		if c.param.Fail {
			fields := logE.Data
			delete(fields, "line_number")
			delete(fields, "new_line")
			delete(fields, "line")
			delete(fields, "workflow_file")
			logE.Data = fields
			logE.Errorf(`action isn't pinned
%s:%d
- %s
+ %s

`, workflowFilePath, i+1, line, l)
		}
	}
	if c.param.Check && failed {
		return ErrActionsNotPinned
	}
	if !changed {
		if failed {
			return ErrActionsNotPinned
		}
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
	if failed || c.param.Fail {
		return ErrActionsNotPinned
	}
	return nil
}

func (c *Controller) readWorkflow(workflowFilePath string) ([]string, error) {
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
