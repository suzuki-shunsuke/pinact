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
	Pattern       string `json:"pattern" jsonschema:"description=A regular expression of target files. If files are passed via positional command line arguments, this is ignored"`
	PatternFormat string `json:"pattern_format" jsonschema:"enum=fixed_string,enum=glob,enum=regexp"`
	patternRegexp *regexp.Regexp
}

func (f *File) Init() error {
	if f.Pattern == "" {
		return errors.New("pattern is required")
	}
	if f.PatternFormat == "" {
		return errors.New("pattern_format is required")
	}
	switch f.PatternFormat {
	case "fixed_string":
		return nil
	case "glob":
		if _, err := path.Match(f.Pattern, "a"); err != nil {
			return fmt.Errorf("parse pattern as a glob: %w", err)
		}
		return nil
	case "regexp":
		r, err := regexp.Compile(f.Pattern)
		if err != nil {
			return fmt.Errorf("compile name as a regular expression: %w", err)
		}
		f.patternRegexp = r
		return nil
	default:
		return errors.New("pattern_format must be fixed_string, glob, or regexp")
	}
}

type IgnoreAction struct {
	Name       string `json:"name" jsonschema:"description=A regular expression to ignore actions and reusable workflows"`
	Ref        string `json:"ref,omitempty" jsonschema:"description=A regular expression to ignore actions and reusable workflows by ref. If not specified, any ref is ignored"`
	NameFormat string `json:"name_format" jsonschema:"enum=fixed_string,enum=glob,enum=regexp"`
	RefFormat  string `json:"ref_format,omitempty" jsonschema:"enum=fixed_string,enum=glob,enum=regexp"`
	nameRegexp *regexp.Regexp
	refRegexp  *regexp.Regexp
}

func (ia *IgnoreAction) Init() error {
	if ia.Name == "" {
		return errors.New("name is required")
	}
	if ia.NameFormat == "" {
		return errors.New("name_format is required")
	}
	switch ia.NameFormat {
	case "fixed_string", "glob":
	case "regexp":
		r, err := regexp.Compile(ia.Name)
		if err != nil {
			return fmt.Errorf("compile name as a regular expression: %w", err)
		}
		ia.nameRegexp = r
	default:
		return errors.New("name_format must be fixed_string, glob, or regexp")
	}
	if ia.Ref != "" {
		if ia.RefFormat == "" {
			return errors.New("ref_format is required if ref is specified")
		}
		switch ia.RefFormat {
		case "fixed_string", "glob":
		case "regexp":
			r, err := regexp.Compile(ia.Ref)
			if err != nil {
				return fmt.Errorf("compile ref as a regular expression: %w", err)
			}
			ia.refRegexp = r
		default:
			return errors.New("ref_format must be fixed_string, glob, or regexp")
		}
	}
	return nil
}

func match(value, name, format string, r *regexp.Regexp) (bool, error) {
	switch format {
	case "fixed_string":
		return value == name, nil
	case "glob":
		return path.Match(value, name)
	case "regexp":
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
