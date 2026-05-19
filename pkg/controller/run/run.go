package run

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/suzuki-shunsuke/go-error-with-exit-code/ecerror"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/urfave"
)

type ParamRun struct {
	WorkflowFilePaths []string
	ConfigFilePath    string
	CWD               string
	IsVerify          bool
	VerifyMinAge      bool
	Update            bool
	IsGitHubActions   bool
	Fix               bool
	NoAPI             bool
	Stderr            io.Writer
	Stdout            io.Writer
	Includes          []*regexp.Regexp
	Excludes          []*regexp.Regexp
	BranchToTags      []*regexp.Regexp
	MinAge            int
	Now               time.Time
	Format            string
	Findings          []Finding
	// DiffFilter, when set, restricts processing to the `+` lines of a
	// unified diff. Files not present in the filter are skipped entirely.
	DiffFilter *DiffFilter
}

// Exit code classes for the v4 spec.
// The Controller accumulates the highest code encountered across all files.
const (
	ExitCodeOK        = 0
	ExitCodeNotPinned = 1 // auto-fixable: semver action that needs SHA pinning
	ExitCodeUnfixable = 2 // not auto-fixable: branch, verify mismatch, min-age violation
	ExitCodeAPIError  = 3 // GitHub API error or unexpected internal error
)

var (
	// ErrActionsNotPinned is returned when a workflow contains actions that need to be pinned.
	// Kept as a public sentinel for external consumers; internally mapped to ExitCodeNotPinned.
	ErrActionsNotPinned = errors.New("action aren't pinned")

	// ErrAPI is returned for GitHub API failures and other unexpected internal errors.
	// Maps to ExitCodeAPIError.
	ErrAPI = errors.New("GitHub API error")

	// ErrUnfixable is returned when an action cannot be auto-fixed (e.g., a branch reference
	// without a matching -branch-to-tag rule, or a verify-comment mismatch).
	// Maps to ExitCodeUnfixable.
	ErrUnfixable = errors.New("action cannot be auto-fixed")

	// ErrMinAge is returned when an action's pinned commit was created after the
	// -min-age cutoff. This is a soft error: the fix (if any) is still applied,
	// but the run exits with ExitCodeUnfixable so CI can flag the violation.
	ErrMinAge = errors.New("action version is younger than the min-age cutoff")
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
		var code int
		var err error
		if c.param.DiffFilter != nil {
			code, err = c.runWorkflowFromDiff(ctx, logger, workflowFilePath)
		} else {
			code, err = c.runWorkflow(ctx, logger, workflowFilePath)
		}
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
		// Wrap ErrSilent so urfave-cli-v3-util suppresses the trailing
		// "pinact failed" log line (per-file errors have already been logged
		// in detail by slogerr.WithError above). ecerror.Wrap surfaces the
		// 0/1/2/3 exit code through ecerror.GetExitCode in main.
		return ecerror.Wrap(urfave.ErrSilent, exitCode)
	}
	return nil
}

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

// runWorkflowFromDiff is the diff-filtered counterpart of runWorkflow: only
// the `+` lines extracted from the unified diff are processed.
//
// I/O is minimised. In check mode (Fix=false) the workflow file is never
// opened. In fix mode the file is opened only if at least one line actually
// changes; otherwise both read and write are skipped.
func (c *Controller) runWorkflowFromDiff(ctx context.Context, logger *slog.Logger, workflowFilePath string) (int, error) {
	exitCode, patches := c.collectDiffPatches(ctx, logger, workflowFilePath)
	if !c.param.Fix || len(patches) == 0 {
		return exitCode, nil
	}
	if err := c.applyPatches(workflowFilePath, patches); err != nil {
		return ExitCodeAPIError, err
	}
	return exitCode, nil
}

// collectDiffPatches runs processLine on every `+` line of the diff for
// workflowFilePath and returns the patches that would change the file.
func (c *Controller) collectDiffPatches(ctx context.Context, logger *slog.Logger, workflowFilePath string) (int, map[int]string) {
	exitCode := ExitCodeOK
	patches := make(map[int]string)
	for _, dl := range c.param.DiffFilter.Lines(workflowFilePath) {
		newLine, changed, code := c.processLine(ctx, logger, workflowFilePath, dl.Number, dl.Content)
		if code > exitCode {
			exitCode = code
		}
		if changed {
			patches[dl.Number] = newLine
		}
	}
	return exitCode, patches
}

// applyPatches reads workflowFilePath, applies patches (keyed by 1-based
// line number) to the corresponding lines, and writes the file back.
func (c *Controller) applyPatches(workflowFilePath string, patches map[int]string) error {
	lines, format, err := c.readWorkflow(workflowFilePath)
	if err != nil {
		return err
	}
	for n, content := range patches {
		if i := n - 1; i >= 0 && i < len(lines) {
			lines[i] = content
		}
	}
	return c.writeWorkflow(workflowFilePath, lines, format)
}

// processLines processes each line in the workflow file.
// It returns whether any lines were changed and the exit code (max of any line's
// contribution: ExitCodeOK / ExitCodeNotPinned / ExitCodeUnfixable / ExitCodeAPIError).
func (c *Controller) processLines(ctx context.Context, logger *slog.Logger, workflowFilePath string, lines []string) (changed bool, exitCode int) {
	for i, lineS := range lines {
		newLine, lineChanged, code := c.processLine(ctx, logger, workflowFilePath, i+1, lineS)
		if lineChanged {
			lines[i] = newLine
			changed = true
		}
		if code > exitCode {
			exitCode = code
		}
	}
	return changed, exitCode
}

// processLine handles a single line of the workflow: parses it, classifies any
// error into an exit code, and reports a patched line back to the caller.
//
// Returns (newLine, changed, exitCode):
//   - newLine: the patched line content when changed is true; empty otherwise
//   - changed: true when the line should be replaced with newLine
//   - exitCode: per-line exit code contribution
//
// processLine has no knowledge of the surrounding file: the caller is
// responsible for applying newLine if it wants to mutate a line buffer.
func (c *Controller) processLine(ctx context.Context, logger *slog.Logger, workflowFilePath string, lineNumber int, lineS string) (newLine string, changed bool, exitCode int) {
	line := &Line{
		File:   workflowFilePath,
		Number: lineNumber,
		Line:   lineS,
	}
	lineLogger := logger.With(
		"line_number", lineNumber,
		"line", lineS,
	)
	l, err := c.parseLine(ctx, lineLogger, lineS)
	if err != nil {
		return c.handleLineError(ctx, lineLogger, line, lineS, l, err)
	}
	if l == "" || lineS == l {
		return "", false, ExitCodeOK
	}
	lineLogger = lineLogger.With("new_line", l)
	// When Fix is disabled, a changed line counts as "needs pinning" since the
	// fix is not being auto-applied. -check and -diff are aliases for
	// -fix=false (handled in di.buildParam), so this single check covers them.
	code := ExitCodeOK
	if !c.param.Fix {
		code = ExitCodeNotPinned
	}
	c.handleChangedLine(ctx, lineLogger, line, l)
	return l, true, code
}

// handleLineError dispatches the per-line error: min-age is a soft error
// (apply the fix anyway, bump exit code), everything else is logged via
// handleParseLineError.
func (c *Controller) handleLineError(ctx context.Context, lineLogger *slog.Logger, line *Line, lineS, l string, err error) (newLine string, changed bool, exitCode int) {
	code := classifyLineError(err)
	if errors.Is(err, ErrMinAge) {
		// Min-age is a soft error: the fix (if any) is still applied,
		// the warning is already in the logger, and exit code is bumped.
		if l != "" && lineS != l {
			c.handleChangedLine(ctx, lineLogger, line, l)
			return l, true, code
		}
		// Already-pinned action whose SHA fails -min-age: no diff to emit,
		// but surface file:line + the line in the human-readable output so
		// the user can see which action triggered the exit-2.
		c.param.Findings = append(c.param.Findings, Finding{
			File:    line.File,
			Line:    line.Number,
			OldLine: line.Line,
			Message: err.Error(),
		})
		c.logger.Output(levelError, "", line, "")
		if c.param.IsGitHubActions {
			fmt.Fprintf(c.param.Stderr, "::error file=%s,line=%d,title=pinact min-age violation::%s\n", line.File, line.Number, err)
		}
		return "", false, code
	}
	c.handleParseLineError(ctx, lineLogger, line, err)
	return "", false, code
}

// classifyLineError maps a per-line parse/process error to an exit code class.
//   - ErrCantPinned / ErrUnfixable / ErrMinAge / verify mismatch -> ExitCodeUnfixable (2)
//   - ErrAPI / anything else                                     -> ExitCodeAPIError  (3)
func classifyLineError(err error) int {
	if errors.Is(err, ErrCantPinned) || errors.Is(err, ErrUnfixable) || errors.Is(err, ErrMinAge) {
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
// It outputs error messages and creates GitHub Actions annotations when applicable.
func (c *Controller) handleParseLineError(_ context.Context, _ *slog.Logger, line *Line, gErr error) {
	// Collect finding for SARIF output
	c.param.Findings = append(c.param.Findings, Finding{
		File:    line.File,
		Line:    line.Number,
		OldLine: line.Line,
		Message: "failed to handle a line: " + gErr.Error(),
	})
	// Output error
	c.logger.Output(levelError, "failed to handle a line: "+gErr.Error(), line, "")
	if c.param.IsGitHubActions {
		fmt.Fprintf(c.param.Stderr, "::error file=%s,line=%d,title=pinact error::%s\n", line.File, line.Number, gErr)
	}
}

// handleChangedLine handles lines that have been modified.
// It collects a SARIF finding, emits the GitHub Actions annotation if running
// in Actions, and prints the detail diff to stderr.
func (c *Controller) handleChangedLine(_ context.Context, _ *slog.Logger, line *Line, newLine string) {
	// Collect finding for SARIF output
	c.param.Findings = append(c.param.Findings, Finding{
		File:    line.File,
		Line:    line.Number,
		OldLine: line.Line,
		NewLine: newLine,
	})
	c.outputGitHubActionsAnnotation(line, newLine)
	c.outputDiff(line, newLine)
}

// outputGitHubActionsAnnotation outputs a GitHub Actions annotation for the changed line.
func (c *Controller) outputGitHubActionsAnnotation(line *Line, newLine string) {
	if !c.param.IsGitHubActions {
		return
	}
	level := "notice"
	if !c.param.Fix {
		level = levelError
	}
	fmt.Fprintf(c.param.Stderr, "::%s file=%s,line=%d,title=pinact error::%s\n", level, line.File, line.Number, newLine)
}

// outputDiff outputs the diff information for the changed line.
// In v4, the detail output is always emitted regardless of -fix / -check / -diff
// flag combinations. Error level is used when the run will exit non-zero
// (Fix disabled), info level otherwise.
func (c *Controller) outputDiff(line *Line, newLine string) {
	level := levelInfo
	if !c.param.Fix {
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
