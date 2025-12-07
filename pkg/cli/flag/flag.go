package flag

import "github.com/urfave/cli/v3"

type GlobalFlags struct {
	LogLevel string
	Config   string
}

func (gf *GlobalFlags) Flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:        "log-level",
			Usage:       "log level",
			Sources:     cli.EnvVars("PINACT_LOG_LEVEL"),
			Destination: &gf.LogLevel,
		},
		&cli.StringFlag{
			Name:        "config",
			Aliases:     []string{"c"},
			Usage:       "configuration file path",
			Sources:     cli.EnvVars("PINACT_CONFIG"),
			Destination: &gf.Config,
		},
	}
}
