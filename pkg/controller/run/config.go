package run

import (
	"fmt"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Files         []*File
	IgnoreActions []*IgnoreAction `yaml:"ignore_actions"`
	IsVerify      bool            `yaml:"-"`
}

type File struct {
	Pattern string
}

type IgnoreAction struct {
	Name string
}

func getConfigPath(fs afero.Fs) (string, error) {
	for _, path := range []string{".pinact.yaml", ".github/pinact.yaml"} {
		f, err := afero.Exists(fs, path)
		if err != nil {
			return "", fmt.Errorf("check if %s exists: %w", path, err)
		}
		if f {
			return path, nil
		}
	}
	return "", nil
}

func (c *Controller) readConfig(configFilePath string, cfg *Config) error {
	var err error
	if configFilePath == "" {
		configFilePath, err = getConfigPath(c.fs)
		if err != nil {
			return err
		}
		if configFilePath == "" {
			return nil
		}
	}
	f, err := c.fs.Open(configFilePath)
	if err != nil {
		return fmt.Errorf("open a configuration file: %w", err)
	}
	defer f.Close()
	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return fmt.Errorf("decode a configuration file as YAML: %w", err)
	}
	return nil
}
