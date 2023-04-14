package cli

import (
	"github.com/suzuki-shunsuke/pinact/pkg/controller/run"
	"github.com/urfave/cli/v2"
)

func (runner *Runner) newRunCommand() *cli.Command {
	return &cli.Command{
		Name:   "run",
		Usage:  "Pin GitHub Actions versions",
		Action: runner.runAction,
	}
}

func (runner *Runner) runAction(c *cli.Context) error {
	ctrl := run.New(c.Context)
	return ctrl.Run(c.Context, runner.LogE, c.Args().Slice()) //nolint:wrapcheck
}
