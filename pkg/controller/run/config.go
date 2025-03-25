package run

import (
	"errors"
	"fmt"
	"path"
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

const (
	formatFixedString = "fixed_string"
	formatGlob        = "glob"
	formatRegexp      = "regexp"
)

func (f *File) Init() error {
	if f.Pattern == "" {
		return errors.New("pattern is required")
	}
	_, err := path.Match(f.Pattern, "a")
	if err != nil {
		return fmt.Errorf("parse pattern as a glob: %w", err)
	}
	return nil
}

type IgnoreAction struct {
	Name       string `json:"name" jsonschema:"description=A regular expression to ignore actions and reusable workflows"`
	Ref        string `json:"ref,omitempty" jsonschema:"description=A regular expression to ignore actions and reusable workflows by ref. If not specified, any ref is ignored"`
	NameFormat string `json:"name_format" jsonschema:"enum=fixed_string,enum=glob,enum=regexp"`
	RefFormat  string `json:"ref_format,omitempty" jsonschema:"enum=fixed_string,enum=glob,enum=regexp"`
	nameRegexp *regexp.Regexp
	refRegexp  *regexp.Regexp
}

func initFormat(value, format string) (*regexp.Regexp, error) {
	switch format {
	case formatFixedString:
		return nil, nil //nolint:nilnil
	case formatGlob:
		if _, err := path.Match(value, "a"); err != nil {
			return nil, fmt.Errorf("parse as a glob: %w", err)
		}
		return nil, nil //nolint:nilnil
	case formatRegexp:
		r, err := regexp.Compile(value)
		if err != nil {
			return nil, fmt.Errorf("compile as a regular expression: %w", err)
		}
		return r, nil
	default:
		return nil, errors.New("name_format must be fixed_string, glob, or regexp")
	}
}

func (ia *IgnoreAction) initName() error {
	if ia.Name == "" {
		return errors.New("name is required")
	}
	if ia.NameFormat == "" {
		return errors.New("name_format is required")
	}
	var err error
	ia.nameRegexp, err = initFormat(ia.Name, ia.NameFormat)
	return err
}

func (ia *IgnoreAction) initRef() error {
	if ia.Ref == "" {
		return nil
	}
	if ia.RefFormat == "" {
		return errors.New("ref_format is required if ref is specified")
	}
	var err error
	ia.refRegexp, err = initFormat(ia.Ref, ia.RefFormat)
	return err
}

func (ia *IgnoreAction) Init() error {
	if err := ia.initName(); err != nil {
		return err
	}
	if err := ia.initRef(); err != nil {
		return err
	}
	return nil
}

func match(value, name, format string, r *regexp.Regexp) (bool, error) {
	switch format {
	case formatFixedString:
		return value == name, nil
	case formatGlob:
		f, err := path.Match(value, name)
		if err != nil {
			return false, fmt.Errorf("match as a glob: %w", err)
		}
		return f, nil
	case formatRegexp:
		return r.MatchString(value), nil
	default:
		return false, errors.New("unexpected format: " + format)
	}
}

func (ia *IgnoreAction) Match(name, ref string) (bool, error) {
	f, err := match(name, ia.Name, ia.NameFormat, ia.nameRegexp)
	if err != nil {
		return false, fmt.Errorf("match name: %w", err)
	}
	if !f {
		return false, nil
	}

	if ia.Ref == "" {
		return true, nil
	}

	f, err = match(ref, ia.Ref, ia.RefFormat, ia.refRegexp)
	if err != nil {
		return false, fmt.Errorf("match ref: %w", err)
	}
	if !f {
		return false, nil
	}
	return true, nil
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

func (c *Controller) readConfig() error {
	cfg := &Config{}
	configFilePath := c.param.ConfigFilePath
	if configFilePath == "" {
		p, err := getConfigPath(c.fs)
		if err != nil {
			return err
		}
		if p == "" {
			return nil
		}
		configFilePath = p
		c.param.ConfigFilePath = configFilePath
	}
	f, err := c.fs.Open(configFilePath)
	if err != nil {
		return fmt.Errorf("open a configuration file: %w", err)
	}
	defer f.Close()
	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return fmt.Errorf("decode a configuration file as YAML: %w", err)
	}
	for _, file := range cfg.Files {
		if err := file.Init(); err != nil {
			return fmt.Errorf("initialize file: %w", err)
		}
	}
	for _, ia := range cfg.IgnoreActions {
		if err := ia.Init(); err != nil {
			return fmt.Errorf("initialize ignore_action: %w", err)
		}
	}
	c.cfg = cfg
	return nil
}
