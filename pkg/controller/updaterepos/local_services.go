package updaterepos

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
)

type GitRunner interface {
	Run(ctx context.Context, dir string, stdout, stderr io.Writer, args ...string) error
	Changed(ctx context.Context, dir string) (bool, error)
	ChangedFiles(ctx context.Context, dir string) ([]string, error)
}

type Pinner interface {
	Pin(ctx context.Context, logger *slog.Logger, workdir string) (bool, error)
}

type localGitRunner struct{ controller *Controller }

func (r localGitRunner) Run(ctx context.Context, dir string, stdout, stderr io.Writer, args ...string) error {
	return r.controller.executeGit(ctx, dir, stdout, stderr, args...)
}

func (r localGitRunner) Changed(ctx context.Context, workdir string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--quiet")
	cmd.Dir = workdir
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("check git diff: %w", err)
}

func (r localGitRunner) ChangedFiles(ctx context.Context, workdir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only")
	cmd.Dir = workdir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("list changed files: %w", err)
	}
	return strings.Fields(stdout.String()), nil
}

type controllerPinner struct{ controller *Controller }

func (p controllerPinner) Pin(ctx context.Context, logger *slog.Logger, workdir string) (bool, error) {
	return p.controller.pin(ctx, logger, workdir)
}
