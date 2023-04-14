package cli

import (
	"github.com/suzuki-shunsuke/pinact/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/pkg/log"
	"github.com/urfave/cli/v2"
)

func (runner *Runner) newRunCommand() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Pin GitHub Actions versions",
		Description: `If no argument is passed, pinact searches GitHub Actions workflow files from .github/workflows.

$ pinact run

You can also pass workflow file paths as arguments.

e.g.

$ pinact run .github/actions/foo/action.yaml .github/actions/bar/action.yaml
`,
		Action: runner.runAction,
	}
}

func (runner *Runner) runAction(c *cli.Context) error {
	ctrl := run.New(c.Context)
	log.SetLevel(c.String("log-level"), runner.LogE)
	return ctrl.Run(c.Context, runner.LogE, c.Args().Slice()) //nolint:wrapcheck
}
