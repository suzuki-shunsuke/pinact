// Package list implements the 'pinact list' command.
// This package provides functionality to list GitHub Actions and reusable workflows
// from workflow files, with support for filtering by owner and custom output formatting.
package list

import (
	"context"
	"fmt"
	"os"
	"regexp"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/flag"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/list"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	"github.com/urfave/cli/v3"
)

// Flags holds the command-line flags for the list command.
type Flags struct {
	Owner        string
	LineTemplate string
	Include      []string
	Exclude      []string
	Args         []string
}

type runner struct{}

// New creates a new list command for the CLI.
// It initializes a runner with the provided logger and returns
// the configured CLI command for listing GitHub Actions.
func New(logger *slogutil.Logger, globalFlags *flag.GlobalFlags) *cli.Command {
	r := &runner{}
	return r.Command(logger, globalFlags)
}

// Command builds and returns the list CLI command configuration.
// It defines all flags, options, and the action handler for the list subcommand.
func (r *runner) Command(logger *slogutil.Logger, globalFlags *flag.GlobalFlags) *cli.Command { //nolint:funlen
	flags := &Flags{}
	return &cli.Command{
		Name:  "list",
		Usage: "List GitHub Actions and reusable workflows",
		Description: `List GitHub Actions and reusable workflows from workflow files.

$ pinact list

Output format (default CSV):
<FilePath>,<LineNumber>,<ActionName>,<Version>,<Comment>

Filter by owner:
$ pinact list --owner actions

Custom output format using Go template:
$ pinact list --line-template "{{.RepoOwner}}/{{.RepoName}}"

Available template fields:
  ActionName - Full action name (e.g., actions/checkout)
  RepoOwner  - Repository owner (e.g., actions)
  RepoName   - Repository name (e.g., checkout)
  Version    - Version/ref (e.g., v4 or commit SHA)
  Comment    - Version comment (e.g., v4.0.0)
  FilePath   - Full file path
  FileName   - Base file name
  LineNumber - Line number in the file
`,
		Action: func(ctx context.Context, _ *cli.Command) error {
			return r.action(ctx, logger, globalFlags, flags)
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "owner",
				Usage:       "Filter actions by owner",
				Destination: &flags.Owner,
			},
			&cli.StringFlag{
				Name:        "line-template",
				Usage:       "Go text/template format for each line",
				Destination: &flags.LineTemplate,
			},
			&cli.StringSliceFlag{
				Name:        "include",
				Aliases:     []string{"i"},
				Usage:       "A regular expression to include actions",
				Destination: &flags.Include,
			},
			&cli.StringSliceFlag{
				Name:        "exclude",
				Aliases:     []string{"e"},
				Usage:       "A regular expression to exclude actions",
				Destination: &flags.Exclude,
			},
		},
		Arguments: []cli.Argument{
			&cli.StringArgs{
				Name:        "files",
				Max:         -1,
				Destination: &flags.Args,
			},
		},
	}
}

func (r *runner) action(ctx context.Context, logger *slogutil.Logger, globalFlags *flag.GlobalFlags, flags *Flags) error {
	if err := logger.SetLevel(globalFlags.LogLevel); err != nil {
		return fmt.Errorf("set log level: %w", err)
	}

	includes, err := compilePatterns(flags.Include)
	if err != nil {
		return fmt.Errorf("compile include patterns: %w", err)
	}

	excludes, err := compilePatterns(flags.Exclude)
	if err != nil {
		return fmt.Errorf("compile exclude patterns: %w", err)
	}

	fs := afero.NewOsFs()
	cfgFilePath, cfg, err := readConfig(fs, globalFlags.Config)
	if err != nil {
		return err
	}

	param := &list.Param{
		WorkflowFilePaths: flags.Args,
		ConfigFilePath:    cfgFilePath,
		Owner:             flags.Owner,
		LineTemplate:      flags.LineTemplate,
		Includes:          includes,
		Excludes:          excludes,
	}

	ctrl := list.New(cfg, param, os.Stdout)
	return ctrl.List(ctx, logger.Logger) //nolint:wrapcheck
}

func readConfig(fs afero.Fs, configFilePath string) (string, *config.Config, error) {
	cfgFinder := config.NewFinder(fs)
	cfgReader := config.NewReader(fs)
	cfgPath, err := cfgFinder.Find(configFilePath)
	if err != nil {
		return "", nil, fmt.Errorf("find configuration file: %w", err)
	}
	cfg := &config.Config{}
	if err := cfgReader.Read(cfg, cfgPath); err != nil {
		return "", nil, fmt.Errorf("read configuration file: %w", err)
	}
	return cfgPath, cfg, nil
}

func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	result := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("compile regex %q: %w", pattern, err)
		}
		result = append(result, re)
	}
	return result, nil
}
