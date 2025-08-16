// Package token implements the 'pinact token' command for secure GitHub token management.
// This package provides functionality to store and retrieve GitHub access tokens
// using the operating system's native credential storage (Windows Credential Manager,
// macOS Keychain, or GNOME Keyring). It offers a secure alternative to environment
// variables for managing authentication credentials, allowing users to persist tokens
// safely across sessions without exposing them in shell configurations.
package token

import (
	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
	ghTokenCLI "github.com/suzuki-shunsuke/urfave-cli-v3-util/keyring/ghtoken/cli"
	"github.com/urfave/cli/v3"
)

// New creates a new token command for the CLI.
// It initializes a GitHub token management command using the system keyring
// for secure credential storage and retrieval.
//
// Parameters:
//   - logE: logrus entry for structured logging
//
// Returns a pointer to the configured CLI command for token operations.
func New(logE *logrus.Entry) *cli.Command {
	return ghTokenCLI.New(logE, github.KeyService)
}
