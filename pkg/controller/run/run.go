package run

import (
	"bytes"
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
	CWD               string
	IsVerify          bool
	Update            bool
	Check             bool
	IsGitHubActions   bool
	Fix               bool
	Diff              bool
	NoAPI             bool
	Stderr            io.Writer
	Stdout            io.Writer
	Review            *Review
	Includes          []*regexp.Regexp
	Excludes          []*regexp.Regexp
	BranchToTags      []*regexp.Regexp
	MinAge            int
	Now               time.Time
	Format            string
	Findings          []Finding
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

// Exit code classes for the v4 spec.
// The Controller accumulates the highest code encountered across all files.
const (
	ExitCodeOK        = 0
	ExitCodeNotPinned = 1 // auto-fixable: semver action that needs SHA pinning
	ExitCodeUnfixable = 2 // not auto-fixable: branch, verify mismatch, min-age violation
	ExitCodeAPIError  = 3 // GitHub API error or unexpected internal error
)

// Run executes the main pinact operation.
// It searches for workflow files and processes each file
// to pin GitHub Actions versions according to the specified parameters.
func (c *Controller) Run(ctx context.Context, logger *slog.Logger) error {
	workflowFilePaths, err := c.searchFiles()
	if err != nil {
		return fmt.Errorf("search target files: %w", err)
	}

	exitCode := ExitCodeOK
	for _, workflowFilePath := range workflowFilePaths {
		logger := logger.With("workflow_file", workflowFilePath)
		code, err := c.runWorkflow(ctx, logger, workflowFilePath)
		if code > exitCode {
			exitCode = code
		}
		if err != nil {
			if code > ExitCodeNotPinned {
				slogerr.WithError(logger, err).Error("process a workflow")
			}
		}
	}
	// Output SARIF if format is sarif
	if c.param.Format == "sarif" {
		if err := c.outputSARIF(); err != nil {
			return fmt.Errorf("output SARIF: %w", err)
		}
	}
	if exitCode > ExitCodeOK {
		// PR1: still return urfave.ErrSilent (binary exit 1).
		// PR4 will replace this with ecerror.Wrap(err, exitCode) to wire 0/1/2/3.
		return urfave.ErrSilent
	}
	return nil
}

// ErrActionsNotPinned is returned when a workflow contains actions that need to be pinned.
// Kept as a public sentinel for external consumers; internally mapped to ExitCodeNotPinned.
var ErrActionsNotPinned = errors.New("action aren't pinned")

// ErrAPI is returned for GitHub API failures and other unexpected internal errors.
// Maps to ExitCodeAPIError.
var ErrAPI = errors.New("GitHub API error")

// ErrUnfixable is returned when an action cannot be auto-fixed (e.g., a branch reference
// without a matching -branch-to-tag rule, or a verify-comment mismatch).
// Maps to ExitCodeUnfixable.
var ErrUnfixable = errors.New("action cannot be auto-fixed")

type Line struct {
	File   string
	Number int
	Line   string
}

// runWorkflow processes a single workflow file.
// It reads the file line by line, parses each line for actions,
// applies transformations, and optionally writes changes back to the file.
// Returns the per-file exit code (max of any line's contribution) and any
// unexpected internal error (file read/write failure).
func (c *Controller) runWorkflow(ctx context.Context, logger *slog.Logger, workflowFilePath string) (int, error) {
	lines, format, err := c.readWorkflow(workflowFilePath)
	if err != nil {
		return ExitCodeAPIError, err
	}
	changed, exitCode := c.processLines(ctx, logger, workflowFilePath, lines)
	if changed && c.param.Fix {
		if err := c.writeWorkflow(workflowFilePath, lines, format); err != nil {
			return ExitCodeAPIError, err
		}
	}
	return exitCode, nil
}

// processLines processes each line in the workflow file.
// It returns whether any lines were changed and the exit code (max of any line's
// contribution: ExitCodeOK / ExitCodeNotPinned / ExitCodeUnfixable / ExitCodeAPIError).
func (c *Controller) processLines(ctx context.Context, logger *slog.Logger, workflowFilePath string, lines []string) (changed bool, exitCode int) {
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
			code := classifyLineError(err)
			if code > exitCode {
				exitCode = code
			}
			c.handleParseLineError(ctx, lineLogger, line, err)
			continue
		}
		if l == "" || lineS == l {
			continue
		}
		lineLogger = lineLogger.With("new_line", l)
		changed = true
		// When Fix is disabled, a line that would be changed counts as
		// "needs pinning" (exit code 1) since we are not auto-applying the fix.
		if !c.param.Fix && exitCode < ExitCodeNotPinned {
			exitCode = ExitCodeNotPinned
		}
		// Backward-compat: the legacy -check flag (handled separately in v3)
		// also signals "needs pinning".
		if c.param.Check && exitCode < ExitCodeNotPinned {
			exitCode = ExitCodeNotPinned
		}
		lines[i] = l
		c.handleChangedLine(ctx, lineLogger, line, l)
	}
	return changed, exitCode
}

// classifyLineError maps a per-line parse/process error to an exit code class.
//   - ErrCantPinned / ErrUnfixable / verify mismatch  -> ExitCodeUnfixable (2)
//   - ErrAPI / anything else                          -> ExitCodeAPIError  (3)
func classifyLineError(err error) int {
	if errors.Is(err, ErrCantPinned) || errors.Is(err, ErrUnfixable) {
		return ExitCodeUnfixable
	}
	return ExitCodeAPIError
}

// fileFormat captures the line-ending style and trailing-newline state of a
// workflow file so they can be preserved across read/write.
type fileFormat struct {
	lineEnding      string // "\n" or "\r\n"
	trailingNewline bool
}

// detectFileFormat inspects the raw file content and returns its line-ending
// style and whether it ends with a newline. The line-ending is determined by
// the first '\n' encountered: if preceded by '\r', CRLF; otherwise LF. Files
// with no '\n' default to LF.
func detectFileFormat(content []byte) *fileFormat {
	f := &fileFormat{lineEnding: "\n"}
	if len(content) == 0 {
		return f
	}
	if i := bytes.IndexByte(content, '\n'); i > 0 && content[i-1] == '\r' {
		f.lineEnding = "\r\n"
	}
	if content[len(content)-1] == '\n' {
		f.trailingNewline = true
	}
	return f
}

// writeWorkflow writes the modified lines back to the workflow file using the
// given file format to preserve the original line endings and trailing-newline
// state.
func (c *Controller) writeWorkflow(workflowFilePath string, lines []string, format *fileFormat) error {
	f, err := c.fs.Create(workflowFilePath)
	if err != nil {
		return fmt.Errorf("create a workflow file: %w", err)
	}
	defer f.Close()
	content := strings.Join(lines, format.lineEnding)
	if format.trailingNewline {
		content += format.lineEnding
	}
	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("write a workflow file: %w", err)
	}
	return nil
}

// handleParseLineError handles errors that occur during line parsing.
// It outputs error messages, creates GitHub Actions annotations, and
// optionally creates pull request review comments.
func (c *Controller) handleParseLineError(ctx context.Context, logger *slog.Logger, line *Line, gErr error) {
	// Collect finding for SARIF output
	c.param.Findings = append(c.param.Findings, Finding{
		File:    line.File,
		Line:    line.Number,
		OldLine: line.Line,
		Message: "failed to handle a line: " + gErr.Error(),
	})
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
		slogerr.WithError(logger, err).Log(
			ctx, level, "create a review comment",
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
	// Collect finding for SARIF output
	c.param.Findings = append(c.param.Findings, Finding{
		File:    line.File,
		Line:    line.Number,
		OldLine: line.Line,
		NewLine: newLine,
	})
	reviewed := c.tryCreateReview(ctx, logger, line, newLine)
	c.outputGitHubActionsAnnotation(line, newLine, reviewed)
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
		slogerr.WithError(logger, err).Log(
			ctx, level, "create a review comment",
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
func (c *Controller) outputGitHubActionsAnnotation(line *Line, newLine string, reviewed bool) {
	if !c.param.IsGitHubActions || reviewed {
		return
	}
	level := "notice"
	if c.param.Check {
		level = levelError
	}
	fmt.Fprintf(c.param.Stderr, "::%s file=%s,line=%d,title=pinact error::%s\n", level, line.File, line.Number, newLine)
}

// outputDiff outputs the diff information for the changed line.
// In v4, the detail output is always emitted regardless of -fix / -check / -diff
// flag combinations. Error level is used when the run will exit non-zero
// (Check or -fix=false), info level otherwise.
func (c *Controller) outputDiff(line *Line, newLine string) {
	level := levelInfo
	if c.param.Check || !c.param.Fix {
		level = levelError
	}
	c.logger.Output(level, "", line, newLine)
}

// readWorkflow reads a workflow file and returns its lines together with its
// detected file format (line-ending style and trailing-newline state). Lines
// are returned without their trailing line-ending characters.
func (c *Controller) readWorkflow(workflowFilePath string) ([]string, *fileFormat, error) {
	content, err := os.ReadFile(workflowFilePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read a workflow file: %w", err)
	}
	format := detectFileFormat(content)
	s := string(content)
	if format.trailingNewline {
		s = strings.TrimSuffix(s, format.lineEnding)
	}
	if s == "" {
		return []string{}, format, nil
	}
	return strings.Split(s, format.lineEnding), format, nil
}
