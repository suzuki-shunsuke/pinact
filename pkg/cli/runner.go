package cli

import (
	"context"
	"io"

	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/pinact/v2/pkg/cli/migrate"
	"github.com/suzuki-shunsuke/urfave-cli-v3-help-all/helpall"
	"github.com/urfave/cli/v3"
)

type Runner struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	LDFlags *LDFlags
	LogE    *logrus.Entry
}

type LDFlags struct {
	Version string
	Commit  string
	Date    string
}

func (r *Runner) Run(ctx context.Context, args ...string) error {
	cmd := helpall.With(&cli.Command{
		Name:    "pinact",
		Usage:   "Pin GitHub Actions versions. https://github.com/suzuki-shunsuke/pinact",
		Version: r.LDFlags.Version + " (" + r.LDFlags.Commit + ")",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log level",
				Sources: cli.EnvVars("PINACT_LOG_LEVEL"),
			},
			&cli.StringFlag{
				Name: "config",
				Aliases: []string{
					"c",
				},
				Usage:   "configuration file path",
				Sources: cli.EnvVars("PINACT_CONFIG"),
			},
		},
		EnableShellCompletion: true,
		Commands: []*cli.Command{
			r.newInitCommand(),
			r.newRunCommand(),
			r.newVersionCommand(),
			migrate.New(r.LogE),
		},
	}, nil)

	return cmd.Run(ctx, args) //nolint:wrapcheck
}
