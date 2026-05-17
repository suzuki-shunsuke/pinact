package initcmd

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

const (
	filePermission os.FileMode = 0o644
	dirPermission  os.FileMode = 0o755
)

//go:embed init.yaml
var templateConfig []byte

// Init creates a new pinact configuration file if it doesn't exist.
// Parent directories are created when needed (e.g. for the global config
// path under ~/.config/pinact/ or %APPDATA%\pinact\).
//
// Returns nil both when the file is newly created and when it already
// exists; callers can stat the path themselves if they need to distinguish.
func (c *Controller) Init(configFilePath string) error {
	f, err := afero.Exists(c.fs, configFilePath)
	if err != nil {
		return fmt.Errorf("check if a configuration file exists: %w", err)
	}
	if f {
		return nil
	}
	if dir := filepath.Dir(configFilePath); dir != "." && dir != "" {
		if err := c.fs.MkdirAll(dir, dirPermission); err != nil {
			return fmt.Errorf("create the parent directory %q: %w", dir, err)
		}
	}
	if err := afero.WriteFile(c.fs, configFilePath, templateConfig, filePermission); err != nil {
		return fmt.Errorf("create a configuration file: %w", err)
	}
	return nil
}
