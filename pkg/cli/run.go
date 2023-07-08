package cli

import (
	"fmt"
	"os"

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
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get the current directory: %w", err)
	}
	param := &run.ParamRun{
		WorkflowFilePaths: c.Args().Slice(),
		ConfigFilePath:    c.String("config"),
		PWD:               pwd,
	}
	return ctrl.Run(c.Context, runner.LogE, param) //nolint:wrapcheck
}
