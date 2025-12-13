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
	"time"

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
	Now               time.Time
}

type Review struct {
	RepoOwner   string
	RepoName    string
	PullRequest int
	SHA         string
}

// Valid checks if the review configuration has all required fields.
// It validates that repo owner, repo name, and pull request number are set,
// returning true if the review configuration is valid for creating reviews.
func (r *Review) Valid() bool {
	return r != nil && r.RepoOwner != "" && r.RepoName != "" && r.PullRequest > 0
}

// Run executes the main pinact operation.
// It searches for workflow files and processes each file
// to pin GitHub Actions versions according to the specified parameters.
func (c *Controller) Run(ctx context.Context, logger *slog.Logger) error {
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
func (c *Controller) runWorkflow(ctx context.Context, logger *slog.Logger, workflowFilePath string) error {
	lines, err := c.readWorkflow(workflowFilePath)
	if err != nil {
		return err
	}
	changed, failed := c.processLines(ctx, logger, workflowFilePath, lines)
	if changed && c.param.Fix {
		if err := c.writeWorkflow(workflowFilePath, lines); err != nil {
			return err
		}
	}
	if failed {
		return ErrActionsNotPinned
	}
	return nil
}

// processLines processes each line in the workflow file.
// It returns whether any lines were changed and whether any errors occurred.
func (c *Controller) processLines(ctx context.Context, logger *slog.Logger, workflowFilePath string, lines []string) (changed, failed bool) {
	for i, lineS := range lines {
		line := &Line{
			File:   workflowFilePath,
			Number: i + 1,
			Line:   lineS,
		}
		lineLogger := logger.With(
			"line_number", i+1,
			"line", lineS,
		)
		l, err := c.parseLine(ctx, lineLogger, lineS)
		if err != nil {
			failed = true
			c.handleParseLineError(ctx, lineLogger, line, err)
			continue
		}
		if l == "" || lineS == l {
			continue
		}
		lineLogger = lineLogger.With("new_line", l)
		changed = true
		if c.param.Check {
			failed = true
		}
		lines[i] = l
		c.handleChangedLine(ctx, lineLogger, line, l)
	}
	return changed, failed
}

// writeWorkflow writes the modified lines back to the workflow file.
func (c *Controller) writeWorkflow(workflowFilePath string, lines []string) error {
	f, err := c.fs.Create(workflowFilePath)
	if err != nil {
		return fmt.Errorf("create a workflow file: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		return fmt.Errorf("write a workflow file: %w", err)
	}
	return nil
}

// handleParseLineError handles errors that occur during line parsing.
// It outputs error messages, creates GitHub Actions annotations, and
// optionally creates pull request review comments.
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
func (c *Controller) handleChangedLine(ctx context.Context, logger *slog.Logger, line *Line, newLine string) {
	reviewed := c.tryCreateReview(ctx, logger, line, newLine)
	c.outputGitHubActionsAnnotation(line, reviewed)
	c.outputDiff(line, newLine)
}

// tryCreateReview attempts to create a PR review comment for the changed line.
// Returns true if the review was created successfully.
func (c *Controller) tryCreateReview(ctx context.Context, logger *slog.Logger, line *Line, newLine string) bool {
	if c.param.Review == nil {
		return false
	}
	code, err := c.review(ctx, line.File, c.param.Review.SHA, line.Number, newLine, nil)
	if err != nil {
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
		return false
	}
	return true
}

// outputGitHubActionsAnnotation outputs a GitHub Actions annotation for the changed line.
func (c *Controller) outputGitHubActionsAnnotation(line *Line, reviewed bool) {
	if !c.param.IsGitHubActions || reviewed {
		return
	}
	level := "notice"
	if c.param.Check {
		level = levelError
	}
	fmt.Fprintf(c.param.Stderr, "::%s file=%s,line=%d,title=pinact error::action isn't pinned\n", level, line.File, line.Number)
}

// outputDiff outputs the diff information for the changed line.
func (c *Controller) outputDiff(line *Line, newLine string) {
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
