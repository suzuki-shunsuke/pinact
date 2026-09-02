// Package gflag holds the global flags of pinact, which every subcommand reads.
package gflag

import (
	"github.com/spf13/pflag"
	"github.com/suzuki-shunsuke/cobra-util/cobrautil"
)

type GlobalFlags struct {
	LogLevel string
	Config   string
}

// Set registers the global flags on fs. fs is the root command's persistent flag
// set, so the flags can be given either before or after the subcommand, as they
// could with urfave/cli, whose command flags were persistent by default.
func (gf *GlobalFlags) Set(fs *pflag.FlagSet) {
	fs.StringVar(&gf.LogLevel, "log-level", "", "log level")
	cobrautil.Envs(fs, "log-level", "PINACT_LOG_LEVEL")
	fs.StringVarP(&gf.Config, "config", "c", "", "configuration file path")
	cobrautil.Envs(fs, "config", "PINACT_CONFIG")
}
