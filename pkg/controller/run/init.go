package run

import (
	"fmt"
	"os"

	"github.com/spf13/afero"
)

const (
	templateConfig = `# yaml-language-server: $schema=https://raw.githubusercontent.com/suzuki-shunsuke/pinact/refs/heads/main/json-schema/pinact.json
# pinact - https://github.com/suzuki-shunsuke/pinact
files:
  - pattern: "^\\.github/workflows/.*\\.ya?ml$"
  - pattern: "^(.*/)?action\\.ya?ml$"

ignore_actions:
# - name: actions/checkout
# - name: slsa-framework/slsa-github-generator/.github/workflows/generator_generic_slsa3.yml
`
	filePermission os.FileMode = 0o644
)

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
