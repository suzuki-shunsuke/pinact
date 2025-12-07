package main

import (
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/urfave"
)

var version = ""

func main() {
	urfave.Main("pinact", version, cli.Run)
}
