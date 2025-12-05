package migrate

import (
	"fmt"
	"log/slog"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"gopkg.in/yaml.v3"
)

// Migrate performs configuration file migration to the latest schema version.
// It finds the configuration file, reads and parses it, determines the required
// migration path, and applies necessary transformations to update the schema.
//
// Parameters:
//   - logger: slog logger for structured logging
//
// Returns an error if migration fails, nil if successful or no migration needed.
func (c *Controller) Migrate(logger *slog.Logger) error {
	// find and read .pinact.yaml
	p, err := c.cfgFinder.Find(c.param.ConfigFilePath)
	if err != nil {
		return fmt.Errorf("find a configurationfile: %w", err)
	}
	if p == "" {
		// if .pinact.yaml doesn't exist, return nil
		logger.Warn("no configuration file is found")
		return nil
	}
	c.param.ConfigFilePath = p

	content, err := afero.ReadFile(c.fs, p)
	if err != nil {
		return fmt.Errorf("read a file: %w", err)
	}

	cfg := &config.Config{}
	if err := yaml.Unmarshal(content, cfg); err != nil {
		return fmt.Errorf("parse a config file: %w", err)
	}
	c.cfg = cfg

	s, err := c.migrate(logger, content)
	if err != nil {
		return err
	}
	if s == "" {
		logger.Info("configuration file isn't changed")
		return nil
	}
	if err := c.edit(c.param.ConfigFilePath, s); err != nil {
		return fmt.Errorf("edit the configuration file: %w", err)
	}
	return nil
}

// edit writes the migrated configuration content back to the file.
// It preserves the original file permissions while updating the content
// with the migrated configuration.
//
// Parameters:
//   - file: path to the configuration file to update
//   - content: migrated configuration content
//
// Returns an error if file operations fail.
func (c *Controller) edit(file, content string) error {
	stat, err := c.fs.Stat(file)
	if err != nil {
		return fmt.Errorf("get configuration file stat: %w", err)
	}
	if err := afero.WriteFile(c.fs, file, []byte(content), stat.Mode()); err != nil {
		return fmt.Errorf("write the configuration file: %w", err)
	}
	return nil
}

// migrate determines and applies the appropriate migration strategy.
// It examines the current configuration version and routes to the
// corresponding migration function.
//
// Parameters:
//   - logger: slog logger for structured logging
//   - content: original configuration file content
//
// Returns the migrated content as string and any error encountered.
func (c *Controller) migrate(logger *slog.Logger, content []byte) (string, error) {
	switch c.cfg.Version {
	case 2: //nolint:mnd
		return c.migrateV2(logger, content)
	case 3: //nolint:mnd
		return "", nil
	case 0:
		return c.migrateEmptyVersion(logger, content)
	default:
		return "", fmt.Errorf("unsupported version: %d", c.cfg.Version)
	}
}

// migrateEmptyVersion migrates configuration files without version information.
// It handles legacy configuration files that don't have explicit version
// fields by applying AST-based migration.
//
// Parameters:
//   - logger: slog logger for structured logging
//   - content: original configuration file content
//
// Returns the migrated content as string and any error encountered.
func (c *Controller) migrateEmptyVersion(logger *slog.Logger, content []byte) (string, error) {
	return parseConfigAST(logger, content)
}

// migrateV2 migrates configuration files from version 2 to version 3.
// It applies necessary transformations to update the schema from version 2
// format to the current version 3 format.
//
// Parameters:
//   - logger: slog logger for structured logging
//   - content: original configuration file content
//
// Returns the migrated content as string and any error encountered.
func (c *Controller) migrateV2(logger *slog.Logger, content []byte) (string, error) {
	// Add code comment
	// Change version from 2 to 3
	// Set name_format and ref_format
	return parseConfigAST(logger, content)
}
