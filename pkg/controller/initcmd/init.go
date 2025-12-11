package initcmd

import (
	"fmt"
	"os"

	"github.com/spf13/afero"
)

const (
	templateConfig = `# yaml-language-server: $schema=https://raw.githubusercontent.com/suzuki-shunsuke/pinact/refs/heads/main/json-schema/pinact.json
# pinact - https://github.com/suzuki-shunsuke/pinact
version: 3
# files:
#   - pattern: action.yaml
#   - pattern: */action.yaml

ignore_actions:
# - name: slsa-framework/slsa-github-generator/\.github/workflows/generator_generic_slsa3\.yml
#   ref: v\d+\.\d+\.\d+
# - name: actions/.*
#   ref: main
# - name: suzuki-shunsuke/.*
#   ref: release-.*
`
	filePermission os.FileMode = 0o644
)

// Init creates a new pinact configuration file if it doesn't exist.
// It checks if the configuration file already exists and creates it with
// a template configuration if it doesn't exist.
//
// Parameters:
//   - configFilePath: path where the configuration file should be created
//
// Returns an error if file operations fail, nil if successful or file already exists.
func (c *Controller) Init(configFilePath string) error {
	f, err := afero.Exists(c.fs, configFilePath)
	if err != nil {
		return fmt.Errorf("check if a configuration file exists: %w", err)
	}
	if f {
		return nil
	}
	if err := afero.WriteFile(c.fs, configFilePath, []byte(templateConfig), filePermission); err != nil {
		return fmt.Errorf("create a configuration file: %w", err)
	}
	return nil
}
