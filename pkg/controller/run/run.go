package run

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
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
	Fix               bool
	Diff              bool
	Stderr            io.Writer
	Review            *Review
	Includes          []*regexp.Regexp
	Excludes          []*regexp.Regexp
}

type Review struct {
	RepoOwner   string
	RepoName    string
	PullRequest int
	SHA         string
}

func (r *Review) Fields() logrus.Fields {
	return logrus.Fields{
		"review_repo_owner": r.RepoOwner,
		"review_repo_name":  r.RepoName,
		"review_pr_number":  r.PullRequest,
		"review_sha":        r.SHA,
	}
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
			failed = true
			if errors.Is(err, ErrActionsNotPinned) {
				continue
			}
			if c.param.Check {
				logerr.WithError(logE, err).Error("check a workflow")
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

type Line struct {
	File   string
	Number int
	Line   string
}

func (c *Controller) runWorkflow(ctx context.Context, logE *logrus.Entry, workflowFilePath string) error { //nolint:cyclop
	lines, err := c.readWorkflow(workflowFilePath)
	if err != nil {
		return err
	}
	changed := false
	failed := false
	for i, lineS := range lines {
		line := &Line{
			File:   workflowFilePath,
			Number: i + 1,
			Line:   lineS,
		}
		logE := logE.WithFields(logrus.Fields{
			"line_number": i + 1,
			"line":        lineS,
		})
		l, err := c.parseLine(ctx, logE, lineS)
		if err != nil {
			failed = true
			c.handleParseLineError(ctx, logE, line, err)
			continue
		}
		if l == "" || lineS == l {
			continue
		}
		logE = logE.WithField("new_line", l)
		changed = true
		if c.param.Check {
			failed = true
		}
		lines[i] = l
		c.handleChangedLine(ctx, logE, line, l)
	}
	// Fix file
	if changed && c.param.Fix {
		f, err := os.Create(workflowFilePath)
		if err != nil {
			return fmt.Errorf("create a workflow file: %w", err)
		}
		defer f.Close()
		if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
			return fmt.Errorf("write a workflow file: %w", err)
		}
	}
	// return error
	if failed {
		return ErrActionsNotPinned
	}
	return nil
}

func (c *Controller) handleParseLineError(ctx context.Context, logE *logrus.Entry, line *Line, gErr error) {
	// Output error
	c.logger.Output(levelError, "failed to handle a line: "+gErr.Error(), line, "")
	if c.param.Review == nil {
		// Output GitHub Actions error
		if c.param.IsGitHubActions {
			fmt.Fprintf(c.param.Stderr, "::error file=%s,line=%d,title=pinact error::%s\n", line.File, line.Number, gErr)
		}
		return
	}
	// Create review
	if code, err := c.review(ctx, line.File, c.param.Review.SHA, line.Number, "", gErr); err != nil {
		level := logrus.ErrorLevel
		if code == http.StatusUnprocessableEntity {
			level = logrus.WarnLevel
		}
		logerr.WithError(logE, err).WithFields(c.param.Review.Fields()).Log(level, "create a review comment")
		// Output GitHub Actions error
		if c.param.IsGitHubActions {
			fmt.Fprintf(c.param.Stderr, "::error file=%s,line=%d,title=pinact error::%s\n", line.File, line.Number, gErr)
		}
	}
}

func (c *Controller) handleChangedLine(ctx context.Context, logE *logrus.Entry, line *Line, newLine string) { //nolint:cyclop
	reviewed := false
	if c.param.Review != nil {
		// Create review
		if code, err := c.review(ctx, line.File, c.param.Review.SHA, line.Number, newLine, nil); err != nil {
			level := logrus.ErrorLevel
			if code == http.StatusUnprocessableEntity {
				level = logrus.WarnLevel
			}
			logerr.WithError(logE, err).WithFields(c.param.Review.Fields()).Log(level, "create a review comment")
		} else {
			reviewed = true
		}
	}
	// Output GitHub Actions error
	if c.param.IsGitHubActions && !reviewed {
		level := "notice"
		if c.param.Check {
			level = levelError
		}
		fmt.Fprintf(c.param.Stderr, "::%s file=%s,line=%d,title=pinact error::action isn't pinned\n", level, line.File, line.Number)
	}
	// Output diff
	if !c.param.Check && c.param.Fix && !c.param.Diff {
		return
	}
	level := "info"
	if c.param.Check {
		level = levelError
	}
	c.logger.Output(level, "action isn't pinned", line, newLine)
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
