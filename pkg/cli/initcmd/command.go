// Package initcmd implements the 'pinact init' command.
// This package is responsible for generating pinact configuration files (.pinact.yaml)
// with default settings to help users quickly set up pinact in their repositories.
// It creates configuration templates that define target workflow files and
// action ignore patterns for the pinning process.
package initcmd

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/cli/gflag"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/controller/initcmd"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/urfave"
	"github.com/urfave/cli/v3"
)

type Flags struct {
	*gflag.GlobalFlags

	Global   bool
	Args     []string
	FirstArg string
}

// New creates a new init command instance with the provided logger.
// It returns a CLI command that can be registered with the main CLI application.
func New(logger *slogutil.Logger, globalFlags *gflag.GlobalFlags, env *urfave.Env) *cli.Command {
	r := &runner{}
	return r.Command(logger, globalFlags, env)
}

type runner struct{}

// Command returns the CLI command definition for the init subcommand.
// It defines the command name, usage, description, and action handler.
func (r *runner) Command(logger *slogutil.Logger, globalFlags *gflag.GlobalFlags, env *urfave.Env) *cli.Command {
	flags := &Flags{GlobalFlags: globalFlags}
	return &cli.Command{
		Name:  "init",
		Usage: "Create a pinact configuration file if it doesn't exist",
		Description: `Create a pinact configuration file if it doesn't exist. The resolved path is printed to stdout.

$ pinact init                          # creates .pinact.yaml in the current directory
$ pinact init .github/pinact.yaml      # explicit path
$ pinact init -g                       # creates the user-wide global config
`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "global",
				Aliases:     []string{"g"},
				Usage:       "Create the user-wide global config file (~/.config/pinact/pinact.yaml on Unix, %APPDATA%\\pinact\\pinact.yaml on Windows). The parent directory is created if it does not exist.",
				Destination: &flags.Global,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			flags.Args = cmd.Args().Slice()
			flags.FirstArg = cmd.Args().First()
			return r.action(ctx, logger, flags, env.Stdout)
		},
	}
}

// action handles the execution of the init command.
// It resolves the target configuration file path (either an explicit argument,
// the user's global config when -g is set, or the default .pinact.yaml),
// delegates creation to the controller, and prints the resolved path to
// stdout so callers can use it in shell scripts.
func (r *runner) action(_ context.Context, logger *slogutil.Logger, flags *Flags, stdout io.Writer) error {
	if err := logger.SetLevel(flags.LogLevel); err != nil {
		return fmt.Errorf("set log level: %w", err)
	}
	configFilePath, err := resolveInitPath(flags)
	if err != nil {
		return err
	}
	ctrl := initcmd.New(afero.NewOsFs())
	if err := ctrl.Init(configFilePath); err != nil {
		return err //nolint:wrapcheck
	}
	fmt.Fprintln(stdout, configFilePath)
	return nil
}

// resolveInitPath determines where the config file should be created.
// Precedence: explicit positional argument > -g (global) > -c global flag >
// default ".pinact.yaml" in the current directory.
func resolveInitPath(flags *Flags) (string, error) {
	if flags.FirstArg != "" {
		return flags.FirstArg, nil
	}
	if flags.Global {
		p := config.GlobalConfigPath()
		if p == "" {
			return "", errors.New("cannot resolve the global config path: APPDATA (Windows) or the home directory is unavailable")
		}
		return p, nil
	}
	if flags.Config != "" {
		return flags.Config, nil
	}
	return ".pinact.yaml", nil
}
