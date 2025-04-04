package migrate

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/migrate"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/log"
	"github.com/urfave/cli/v3"
)

type Runner struct {
	LogE *logrus.Entry
}

func New(logE *logrus.Entry) *cli.Command {
	r := Runner{
		LogE: logE,
	}
	return r.newCommand()
}

func (r *Runner) newCommand() *cli.Command {
	return &cli.Command{
		Name:  "migrate",
		Usage: "Migrate .pinact.yaml",
		Description: `Migrate the version of .pinact.yaml

$ pinact migrate
`,
		Action: r.action,
	}
}

func (r *Runner) action(_ context.Context, c *cli.Command) error {
	log.SetLevel(c.String("log-level"), r.LogE)
	fs := afero.NewOsFs()
	ctrl := migrate.New(fs, config.NewFinder(fs), &migrate.Param{
		ConfigFilePath: c.String("config"),
	})

	return ctrl.Migrate(r.LogE) //nolint:wrapcheck
}
