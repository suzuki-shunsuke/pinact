package cli

import (
	"context"
	"io"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/urfave-cli-help-all/helpall"
	"github.com/urfave/cli/v2"
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
	compiledDate, err := time.Parse(time.RFC3339, r.LDFlags.Date)
	if err != nil {
		compiledDate = time.Now()
	}
	app := cli.App{
		Name:     "pinact",
		Usage:    "Pin GitHub Actions versions. https://github.com/suzuki-shunsuke/pinact",
		Version:  r.LDFlags.Version + " (" + r.LDFlags.Commit + ")",
		Compiled: compiledDate,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log level",
				EnvVars: []string{"PINACT_LOG_LEVEL"},
			},
			&cli.StringFlag{
				Name: "config",
				Aliases: []string{
					"c",
				},
				Usage:   "configuration file path",
				EnvVars: []string{"PINACT_CONFIG"},
			},
		},
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			r.newInitCommand(),
			r.newRunCommand(),
			r.newVersionCommand(),
			helpall.New(nil),
		},
	}

	return app.RunContext(ctx, args) //nolint:wrapcheck
}
