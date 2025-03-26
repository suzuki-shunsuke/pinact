package cli

import (
	"fmt"
	"os"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v2/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/v2/pkg/github"
	"github.com/suzuki-shunsuke/pinact/v2/pkg/log"
	"github.com/urfave/cli/v2"
)

func (r *Runner) newInitCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Create .pinact.yaml if it doesn't exist",
		Description: `Create .pinact.yaml if it doesn't exist

$ pinact init

You can also pass configuration file path.

e.g.

$ pinact init .github/pinact.yaml
`,
		Action: r.initAction,
	}
}

func (r *Runner) initAction(c *cli.Context) error {
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get the current directory: %w", err)
	}
	gh := github.New(c.Context)
	ctrl := run.New(&run.RepositoriesServiceImpl{
		Tags:                map[string]*run.ListTagsResult{},
		Releases:            map[string]*run.ListReleasesResult{},
		Commits:             map[string]*run.GetCommitSHA1Result{},
		RepositoriesService: gh.Repositories,
	}, afero.NewOsFs(), nil, nil, &run.ParamRun{
		WorkflowFilePaths: c.Args().Slice(),
		ConfigFilePath:    c.String("config"),
		PWD:               pwd,
		IsVerify:          c.Bool("verify"),
		Check:             c.Bool("check"),
		Update:            c.Bool("update"),
	})

	log.SetLevel(c.String("log-level"), r.LogE)
	configFilePath := c.Args().First()
	if configFilePath == "" {
		configFilePath = c.String("config")
	}
	if configFilePath == "" {
		configFilePath = ".pinact.yaml"
	}
	return ctrl.Init(configFilePath) //nolint:wrapcheck
}
