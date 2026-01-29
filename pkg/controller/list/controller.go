// Package list implements the 'pinact list' command.
// This package provides functionality to list GitHub Actions and reusable workflows
// from workflow files, with support for filtering by owner and custom output formatting.
package list

import (
	"io"
	"regexp"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
)

// Controller handles the list command operations.
type Controller struct {
	cfg    *config.Config
	param  *Param
	stdout io.Writer
}

// Param contains parameters for the list command.
type Param struct {
	WorkflowFilePaths []string
	ConfigFilePath    string
	Owner             string
	LineTemplate      string
	Includes          []*regexp.Regexp
	Excludes          []*regexp.Regexp
}

// New creates a new Controller for running list operations.
func New(cfg *config.Config, param *Param, stdout io.Writer) *Controller {
	return &Controller{
		cfg:    cfg,
		param:  param,
		stdout: stdout,
	}
}
