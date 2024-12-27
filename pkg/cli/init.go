package cli

import (
	"github.com/suzuki-shunsuke/pinact/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/pkg/log"
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
	ctrl := run.New(c.Context, &run.InputNew{})
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
