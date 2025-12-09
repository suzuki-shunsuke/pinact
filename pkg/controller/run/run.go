package run

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/urfave"
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
	MinAge            int
}

type Review struct {
	RepoOwner   string
	RepoName    string
	PullRequest int
	SHA         string
}

// Valid checks if the review configuration has all required fields.
// It validates that repo owner, repo name, and pull request number are set.
//
// Returns true if the review configuration is valid for creating reviews.
func (r *Review) Valid() bool {
	return r != nil && r.RepoOwner != "" && r.RepoName != "" && r.PullRequest > 0
}

// Run executes the main pinact operation.
// It reads configuration, searches for workflow files, and processes each file
// to pin GitHub Actions versions according to the specified parameters.
// Returns an error if the operation fails or actions are not pinned in check mode.
func (c *Controller) Run(ctx context.Context, logger *slog.Logger) error {
	if err := c.readConfig(); err != nil {
		return err
	}
	workflowFilePaths, err := c.searchFiles()
	if err != nil {
		return fmt.Errorf("search target files: %w", err)
	}

	failed := false
	for _, workflowFilePath := range workflowFilePaths {
		logger := logger.With("workflow_file", workflowFilePath)
		if err := c.runWorkflow(ctx, logger, workflowFilePath); err != nil {
			failed = true
			if errors.Is(err, ErrActionsNotPinned) {
				continue
			}
			if c.param.Check {
				slogerr.WithError(logger, err).Error("check a workflow")
				continue
			}
			slogerr.WithError(logger, err).Error("update a workflow")
		}
	}
	if failed {
		return urfave.ErrSilent
	}
	return nil
}

// readConfig loads and processes the pinact configuration file.
// It finds the configuration file path and reads the configuration,
// updating the controller's configuration state.
//
// Returns an error if configuration cannot be found or read.
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

// runWorkflow processes a single workflow file.
// It reads the file line by line, parses each line for actions,
// applies transformations, and optionally writes changes back to the file.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - logger: slog logger for structured logging
//   - workflowFilePath: path to the workflow file to process
//
// Returns an error if processing fails or actions are not pinned in check mode.
func (c *Controller) runWorkflow(ctx context.Context, logger *slog.Logger, workflowFilePath string) error { //nolint:cyclop
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
		logger := logger.With(
			"line_number", i+1,
			"line", lineS,
		)
		l, err := c.parseLine(ctx, logger, lineS)
		if err != nil {
			failed = true
			c.handleParseLineError(ctx, logger, line, err)
			continue
		}
		if l == "" || lineS == l {
			continue
		}
		logger = logger.With("new_line", l)
		changed = true
		if c.param.Check {
			failed = true
		}
		lines[i] = l
		c.handleChangedLine(ctx, logger, line, l)
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

// handleParseLineError handles errors that occur during line parsing.
// It outputs error messages, creates GitHub Actions annotations, and
// optionally creates pull request review comments.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - logger: slog logger for structured logging
//   - line: line information where the error occurred
//   - gErr: error that occurred during parsing
func (c *Controller) handleParseLineError(ctx context.Context, logger *slog.Logger, line *Line, gErr error) {
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
		level := slog.LevelError
		if code == http.StatusUnprocessableEntity {
			level = slog.LevelWarn
		}
		slogerr.WithError(logger, err).Log(ctx, level, "create a review comment",
			"review_repo_owner", c.param.Review.RepoOwner,
			"review_repo_name", c.param.Review.RepoName,
			"review_pr_number", c.param.Review.PullRequest,
			"review_sha", c.param.Review.SHA,
		)
		// Output GitHub Actions error
		if c.param.IsGitHubActions {
			fmt.Fprintf(c.param.Stderr, "::error file=%s,line=%d,title=pinact error::%s\n", line.File, line.Number, gErr)
		}
	}
}

// handleChangedLine handles lines that have been modified.
// It creates review comments, GitHub Actions annotations, and outputs
// diff information depending on the operation mode.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - logger: slog logger for structured logging
//   - line: original line information
//   - newLine: modified line content
func (c *Controller) handleChangedLine(ctx context.Context, logger *slog.Logger, line *Line, newLine string) { //nolint:cyclop
	reviewed := false
	if c.param.Review != nil {
		// Create review
		if code, err := c.review(ctx, line.File, c.param.Review.SHA, line.Number, newLine, nil); err != nil {
			level := slog.LevelError
			if code == http.StatusUnprocessableEntity {
				level = slog.LevelWarn
			}
			slogerr.WithError(logger, err).Log(ctx, level, "create a review comment",
				"review_repo_owner", c.param.Review.RepoOwner,
				"review_repo_name", c.param.Review.RepoName,
				"review_pr_number", c.param.Review.PullRequest,
				"review_sha", c.param.Review.SHA,
			)
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

// readWorkflow reads a workflow file and returns its lines.
// It opens the file and scans it line by line, returning all lines
// as a slice of strings.
//
// Parameters:
//   - workflowFilePath: path to the workflow file to read
//
// Returns a slice of lines from the file and any error encountered.
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
