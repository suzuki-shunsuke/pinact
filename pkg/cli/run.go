package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/log"
	"github.com/urfave/cli/v3"
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

func (r *Runner) runAction(ctx context.Context, c *cli.Command) error {
	log.SetLevel(c.String("log-level"), r.LogE)
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get the current directory: %w", err)
	}

	gh := github.New(ctx)
	fs := afero.NewOsFs()
	ctrl := run.New(&run.RepositoriesServiceImpl{
		Tags:                map[string]*run.ListTagsResult{},
		Releases:            map[string]*run.ListReleasesResult{},
		Commits:             map[string]*run.GetCommitSHA1Result{},
		RepositoriesService: gh.Repositories,
	}, fs, config.NewFinder(fs), config.NewReader(fs), &run.ParamRun{
		WorkflowFilePaths: c.Args().Slice(),
		ConfigFilePath:    c.String("config"),
		PWD:               pwd,
		IsVerify:          c.Bool("verify"),
		Check:             c.Bool("check"),
		Update:            c.Bool("update"),
	})
	return ctrl.Run(ctx, r.LogE) //nolint:wrapcheck
}
