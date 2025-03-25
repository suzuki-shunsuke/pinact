package migrate

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/pkg/config"
	"gopkg.in/yaml.v3"
)

func (c *Controller) Migrate(logE *logrus.Entry) error {
	// find and read .pinact.yaml
	p, err := c.cfgFinder.Find(c.param.ConfigFilePath)
	if err != nil {
		return fmt.Errorf("find a configurationfile: %w", err)
	}
	if p == "" {
		// if .pinact.yaml doesn't exist, return nil
		logE.Warn("no configuration file is found")
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

	s, err := c.migrate(logE, content)
	if err != nil {
		return err
	}
	if s == "" {
		logE.Info("configuration file isn't changed")
		return nil
	}
	if err := c.edit(c.param.ConfigFilePath, s); err != nil {
		return fmt.Errorf("edit the configuration file: %w", err)
	}
	return nil
}

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

func (c *Controller) migrate(logE *logrus.Entry, content []byte) (string, error) {
	switch c.cfg.Version {
	case 2:
		return c.migrateV2(logE, content)
	case 3:
		return "", nil
	case 0:
		return c.migrateEmptyVersion(logE, content)
	default:
		return "", fmt.Errorf("unsupported version: %d", c.cfg.Version)
	}
}

func (c *Controller) migrateEmptyVersion(logE *logrus.Entry, content []byte) (string, error) {
	return parseConfigAST(logE, content)
}

func (c *Controller) migrateV2(logE *logrus.Entry, content []byte) (string, error) {
	// Add code comment
	// Change version from 2 to 3
	// Set name_format and ref_format
	return parseConfigAST(logE, content)
}
