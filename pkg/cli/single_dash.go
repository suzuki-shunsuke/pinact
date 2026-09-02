package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// checkSingleDashLongFlags rejects a long flag written with a single dash, such as
// "pinact run -check".
//
// urfave/cli accepted that form and pinact's own documentation used it, but pflag
// reads it as a cluster of shorthands: "-check" is "-c heck", so it silently sets
// --config to "heck" instead of enabling --check. Failing here turns what would be a
// confusing run against the wrong config file into an error naming the form to use.
//
// A name is reported only when it is a long flag somewhere in the command tree, so a
// real cluster of shorthands ("-uv") and a shorthand with an attached value ("-m7")
// are left to pflag.
func checkSingleDashLongFlags(cmd *cobra.Command, args []string) error {
	longFlags := longFlagNames(cmd)
	for _, arg := range args {
		if arg == "--" {
			// Everything after the terminator is a positional argument.
			return nil
		}
		name, ok := singleDashName(arg)
		if !ok {
			continue
		}
		if _, ok := longFlags[name]; ok {
			return fmt.Errorf("unknown flag: %s. Long flags need two dashes since pinact v5: --%s", arg, name)
		}
	}
	return nil
}

// singleDashName returns the flag name of an argument written as a single dash
// followed by more than one character, which is the only form that can collide with a
// long flag. The value of "-name=value" is dropped, since only the name is matched.
func singleDashName(arg string) (string, bool) {
	if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") {
		return "", false
	}
	name, _, _ := strings.Cut(strings.TrimPrefix(arg, "-"), "=")
	if len(name) < 2 { //nolint:mnd // A single character is a shorthand, which is valid.
		return "", false
	}
	return name, true
}

// longFlagNames collects the long flags of cmd and of every command below it. The
// whole tree is collected rather than the flags of the command being run, because the
// command is not resolved until pflag has parsed the arguments this check precedes.
func longFlagNames(cmd *cobra.Command) map[string]struct{} {
	names := map[string]struct{}{}
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		collect := func(f *pflag.Flag) {
			names[f.Name] = struct{}{}
		}
		c.Flags().VisitAll(collect)
		c.PersistentFlags().VisitAll(collect)
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(cmd)
	return names
}
