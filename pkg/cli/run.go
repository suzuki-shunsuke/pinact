package cli

import (
	"fmt"
	"os"

	"github.com/suzuki-shunsuke/pinact/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/pkg/log"
	"github.com/urfave/cli/v2"
)

func (r *Runner) newRunCommand() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Pin GitHub Actions versions",
		Description: `If no argument is passed, pinact searches GitHub Actions workflow files from .github/workflows.

$ pinact run

You can also pass workflow file paths as arguments.

e.g.

$ pinact run .github/actions/foo/action.yaml .github/actions/bar/action.yaml
`,
		Action: r.runAction,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verify",
				Aliases: []string{"v"},
				Usage:   "Verify if pairs of commit SHA and version are correct",
			},
			&cli.BoolFlag{
				Name:  "check",
				Usage: "Exit with a non-zero status code if actions are not pinned. If this is true, files aren't updated",
			},
			&cli.BoolFlag{
				Name:    "update",
				Aliases: []string{"u"},
				Usage:   "Update actions to latest versions",
			},
		},
	}
}

func (r *Runner) runAction(c *cli.Context) error {
	ctrl := run.New(c.Context, &run.InputNew{
		Update: c.Bool("update"),
	})
	log.SetLevel(c.String("log-level"), r.LogE)
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get the current directory: %w", err)
	}
	param := &run.ParamRun{
		WorkflowFilePaths: c.Args().Slice(),
		ConfigFilePath:    c.String("config"),
		PWD:               pwd,
		IsVerify:          c.Bool("verify"),
		Check:             c.Bool("check"),
	}
	return ctrl.Run(c.Context, r.LogE, param) //nolint:wrapcheck
}
