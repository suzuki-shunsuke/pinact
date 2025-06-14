package token

import (
	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
	ghTokenCLI "github.com/suzuki-shunsuke/urfave-cli-v3-util/keyring/ghtoken/cli"
	"github.com/urfave/cli/v3"
)

func New(logE *logrus.Entry) *cli.Command {
	return ghTokenCLI.New(logE, github.KeyService)
}
