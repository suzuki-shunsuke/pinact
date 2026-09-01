package main

import (
	"github.com/suzuki-shunsuke/cobra-util/cobrautil"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli"
)

var version = ""

func main() {
	cobrautil.Main("pinact", version, cli.Run)
}
