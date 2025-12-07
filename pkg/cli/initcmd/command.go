// Package initcmd implements the 'pinact init' command.
// This package is responsible for generating pinact configuration files (.pinact.yaml)
// with default settings to help users quickly set up pinact in their repositories.
// It creates configuration templates that define target workflow files and
// action ignore patterns for the pinning process.
package initcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/urfave"
	"github.com/urfave/cli/v3"
)

// New creates a new init command instance with the provided logger.
// It returns a CLI command that can be registered with the main CLI application.
func New(logger *slogutil.Logger) *cli.Command {
	r := &runner{}
	return r.Command(logger)
}

type runner struct{}

// Command returns the CLI command definition for the init subcommand.
// It defines the command name, usage, description, and action handler.
func (r *runner) Command(logger *slogutil.Logger) *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Create .pinact.yaml if it doesn't exist",
		Description: `Create .pinact.yaml if it doesn't exist

$ pinact init

You can also pass configuration file path.

e.g.

$ pinact init .github/pinact.yaml
`,
		Action: urfave.Action(r.action, logger),
	}
}

// action handles the execution of the init command.
// It creates a default .pinact.yaml configuration file in the specified location.
// The function sets up the necessary controllers and services, determines the output
// path for the configuration file, and delegates to the controller's Init method.
func (r *runner) action(ctx context.Context, c *cli.Command, logger *slogutil.Logger) error {
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get the current directory: %w", err)
	}
	gh := github.New(ctx, logger.Logger)
	ctrl := run.New(&run.RepositoriesServiceImpl{
		Tags:                map[string]*run.ListTagsResult{},
		Releases:            map[string]*run.ListReleasesResult{},
		Commits:             map[string]*run.GetCommitSHA1Result{},
		RepositoriesService: gh.Repositories,
	}, gh.PullRequests, afero.NewOsFs(), nil, nil, &run.ParamRun{
		WorkflowFilePaths: c.Args().Slice(),
		ConfigFilePath:    c.String("config"),
		PWD:               pwd,
		IsVerify:          c.Bool("verify"),
		Check:             c.Bool("check"),
		Update:            c.Bool("update"),
	})

	if err := logger.SetLevel(c.String("log-level")); err != nil {
		return fmt.Errorf("set log level: %w", err)
	}
	configFilePath := c.Args().First()
	if configFilePath == "" {
		configFilePath = c.String("config")
	}
	if configFilePath == "" {
		configFilePath = ".pinact.yaml"
	}
	return ctrl.Init(configFilePath) //nolint:wrapcheck
}
