package initcmd

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/spf13/afero"
)

const filePermission os.FileMode = 0o644

//go:embed init.yaml
var templateConfig []byte

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
	if err := afero.WriteFile(c.fs, configFilePath, templateConfig, filePermission); err != nil {
		return fmt.Errorf("create a configuration file: %w", err)
	}
	return nil
}
