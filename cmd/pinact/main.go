package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/log"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/urfave"
)

var (
	version = ""
	commit  = "" //nolint:gochecknoglobals
	date    = "" //nolint:gochecknoglobals
)

type HasExitCode interface {
	ExitCode() int
}

func main() {
	logE := log.New(version)
	if err := core(logE); err != nil {
		if errors.Is(err, run.ErrActionsNotPinned) {
			os.Exit(1)
		}
		logerr.WithError(logE, err).Fatal("pinact failed")
	}
}

func core(logE *logrus.Entry) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return cli.Run(ctx, logE, &urfave.LDFlags{ //nolint:wrapcheck
		Version: version,
		Commit:  commit,
		Date:    date,
	}, os.Args...)
}
