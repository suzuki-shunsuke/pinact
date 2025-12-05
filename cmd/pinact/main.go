package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/suzuki-shunsuke/go-stdutil"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/run"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
)

var (
	version = ""
	commit  = "" //nolint:gochecknoglobals
	date    = "" //nolint:gochecknoglobals
)

func main() {
	if code := core(); code != 0 {
		os.Exit(code)
	}
}

func core() int {
	logLevelVar := &slog.LevelVar{}
	logger := slogutil.New(&slogutil.InputNew{
		Name:    "pinact",
		Version: version,
		Out:     os.Stderr,
		Level:   logLevelVar,
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := cli.Run(ctx, logger, logLevelVar, &stdutil.LDFlags{
		Version: version,
		Commit:  commit,
		Date:    date,
	}, os.Args...); err != nil {
		if errors.Is(err, run.ErrActionsNotPinned) {
			return 1
		}
		slogerr.WithError(logger, err).Error("pinact failed")
		return 1
	}
	return 0
}
