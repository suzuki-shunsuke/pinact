package cli

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/initcmd"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/migrate"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/run"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/urfave"
	"github.com/urfave/cli/v3"
)

func Run(ctx context.Context, logE *logrus.Entry, ldFlags *urfave.LDFlags, args ...string) error {
	return urfave.Command(logE, ldFlags, &cli.Command{ //nolint:wrapcheck
		Name:  "pinact",
		Usage: "Pin GitHub Actions versions. https://github.com/suzuki-shunsuke/pinact",
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
		Commands: []*cli.Command{
			initcmd.New(logE),
			run.New(logE),
			migrate.New(logE),
		},
	}).Run(ctx, args)
}
