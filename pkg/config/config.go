package config

import (
	"errors"
	"fmt"
	"path"
	"regexp"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Version       int             `json:"version,omitempty" jsonschema:"enum=2,enum=3"`
	Files         []*File         `json:"files,omitempty" jsonschema:"description=Target files. If files are passed via positional command line arguments, this is ignored"`
	IgnoreActions []*IgnoreAction `json:"ignore_actions,omitempty" yaml:"ignore_actions" jsonschema:"description=Actions and reusable workflows that pinact ignores"`
}

type File struct {
	Pattern string `json:"pattern"`
}

var (
	errUnsupportedConfigVersion = errors.New("pinact doesn't suuport this configuration format version. Maybe you need to update pinact")
	errAbandonedConfigVersion   = errors.New("this version was abandoned. Pleaes update the scheme version")
	errEmptyConfigVersion       = errors.New("schema version is required")
)

func validateSchemaVersion(v int) error {
	switch v {
	case 0:
		return errEmptyConfigVersion
	case 2: //nolint:mnd
		return errAbandonedConfigVersion
	case 3: //nolint:mnd
		return nil
	default:
		return errUnsupportedConfigVersion
	}
}

func (f *File) Init(v int) error {
	if f.Pattern == "" {
		return errors.New("pattern is required")
	}
	if err := validateSchemaVersion(v); err != nil {
		return err
	}
	_, err := path.Match(f.Pattern, "a")
	if err != nil {
		return fmt.Errorf("parse pattern as a glob: %w", err)
	}
	return nil
}

type IgnoreAction struct {
	Name       string `json:"name"`
	Ref        string `json:"ref,omitempty"`
	nameRegexp *regexp.Regexp
	refRegexp  *regexp.Regexp
}

func (ia *IgnoreAction) initName() error {
	if ia.Name == "" {
		return errors.New("name is required")
	}
	r, err := regexp.Compile(ia.Name)
	if err != nil {
		return fmt.Errorf("compile name as a regular expression: %w", err)
	}
	ia.nameRegexp = r
	return nil
}

func (ia *IgnoreAction) initRef(v int) error {
	if err := validateSchemaVersion(v); err != nil {
		return err
	}
	if ia.Ref == "" {
		return errors.New("ref is required")
	}
	r, err := regexp.Compile(ia.Ref)
	if err != nil {
		return fmt.Errorf("compile ref as a regular expression: %w", err)
	}
	ia.refRegexp = r
	return nil
}

func (ia *IgnoreAction) Init(v int) error {
	if err := ia.initName(); err != nil {
		return err
	}
	if err := ia.initRef(v); err != nil {
		return err
	}
	return nil
}

func (ia *IgnoreAction) matchName(name string, version int) (bool, error) {
	if err := validateSchemaVersion(version); err != nil {
		return false, err
	}
	return ia.nameRegexp.FindString(name) == name, nil
}

func (ia *IgnoreAction) matchRef(ref string, version int) (bool, error) {
	if err := validateSchemaVersion(version); err != nil {
		return false, err
	}
	return ia.refRegexp.FindString(ref) == ref, nil
}

func (ia *IgnoreAction) Match(name, ref string, version int) (bool, error) {
	f, err := ia.matchName(name, version)
	if err != nil {
		return false, fmt.Errorf("match name: %w", err)
	}
	if !f {
		return false, nil
	}
	b, err := ia.matchRef(ref, version)
	if err != nil {
		return false, fmt.Errorf("match ref: %w", err)
	}
	return b, nil
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

type Finder struct {
	fs afero.Fs
}

func NewFinder(fs afero.Fs) *Finder {
	return &Finder{fs: fs}
}

func (f *Finder) Find(configFilePath string) (string, error) {
	if configFilePath != "" {
		return configFilePath, nil
	}
	p, err := getConfigPath(f.fs)
	if err != nil {
		return "", err
	}
	return p, nil
}

type Reader struct {
	fs afero.Fs
}

func NewReader(fs afero.Fs) *Reader {
	return &Reader{fs: fs}
}

func (r *Reader) Read(cfg *Config, configFilePath string) error {
	if configFilePath == "" {
		return nil
	}
	f, err := r.fs.Open(configFilePath)
	if err != nil {
		return fmt.Errorf("open a configuration file: %w", err)
	}
	defer f.Close()
	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return fmt.Errorf("decode a configuration file as YAML: %w", err)
	}
	if err := validateSchemaVersion(cfg.Version); err != nil {
		return err
	}
	for _, file := range cfg.Files {
		if err := file.Init(cfg.Version); err != nil {
			return fmt.Errorf("initialize file: %w", err)
		}
	}
	for _, ia := range cfg.IgnoreActions {
		if err := ia.Init(cfg.Version); err != nil {
			return fmt.Errorf("initialize ignore_action: %w", err)
		}
	}
	return nil
}
