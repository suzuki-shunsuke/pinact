package run

import (
	"fmt"
	"regexp"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Files         []*File         `json:"files,omitempty" jsonschema:"description=Target files. If files are passed via positional command line arguments, this is ignored"`
	IgnoreActions []*IgnoreAction `json:"ignore_actions,omitempty" yaml:"ignore_actions" jsonschema:"description=Actions and reusable workflows that pinact ignores"`
	IsVerify      bool            `json:"-" yaml:"-"`
	Check         bool            `json:"-" yaml:"-"`
}

type File struct {
	Pattern string `json:"pattern" jsonschema:"description=A regular expression of target files. If files are passed via positional command line arguments, this is ignored"`
}

type IgnoreAction struct {
	Name      string `json:"name" jsonschema:"description=A regular expression to ignore actions and reusable workflows"`
	Ref       string `json:"ref,omitempty" jsonschema:"description=A regular expression to ignore actions and reusable workflows by ref. If not specified, any ref is ignored"`
	regexp    *regexp.Regexp
	refRegexp *regexp.Regexp
}

func NewIgnoreAction(name, ref string) (*IgnoreAction, error) {
	ia := &IgnoreAction{
		Name: name,
		Ref:  ref,
	}

	var err error
	ia.regexp, err = regexp.Compile(name)
	if err != nil {
		return nil, fmt.Errorf("compile a regular expression: %w", err)
	}

	if ref != "" {
		ia.refRegexp, err = regexp.Compile(ref)
		if err != nil {
			return nil, fmt.Errorf("compile a regular expression for ref: %w", err)
		}
	}

	return ia, nil
}

func (ia *IgnoreAction) Match(name, ref string) bool {
	if !ia.regexp.MatchString(name) {
		return false
	}
	if ia.Ref == "" {
		return true
	}
	return ia.refRegexp.MatchString(ref)
}

func getConfigPath(fs afero.Fs) (string, error) {
	for _, path := range []string{".pinact.yaml", ".github/pinact.yaml", ".pinact.yml", ".github/pinact.yml"} {
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
	for i, ignoreAction := range cfg.IgnoreActions {
		ia, err := NewIgnoreAction(ignoreAction.Name, ignoreAction.Ref)
		if err != nil {
			return fmt.Errorf("create ignore action %d: %w", i, err)
		}
		cfg.IgnoreActions[i] = ia
	}
	return nil
}
