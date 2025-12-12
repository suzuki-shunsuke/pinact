// Package config manages pinact configuration files and validation.
// This package is responsible for reading, parsing, and validating .pinact.yaml
// configuration files. It handles multiple schema versions, manages file patterns
// for targeting specific workflow files, and maintains ignore rules for excluding
// certain actions from the pinning process. The package provides interfaces for
// finding and reading configuration files from standard locations, ensuring
// backward compatibility while supporting schema evolution.
package config

import (
	"errors"
	"fmt"
	"os"
	"path"
	"regexp"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Version       int             `json:"version,omitempty" jsonschema:"enum=2,enum=3"`
	Files         []*File         `json:"files,omitempty" jsonschema:"description=Target files. If files are passed via positional command line arguments, this is ignored"`
	IgnoreActions []*IgnoreAction `json:"ignore_actions,omitempty" yaml:"ignore_actions" jsonschema:"description=Actions and reusable workflows that pinact ignores"`
	GHES          *GHES           `json:"ghes,omitempty" yaml:"ghes" jsonschema:"description=GitHub Enterprise Server configuration"`
}

type GHES struct {
	APIURL string `json:"api_url,omitempty" yaml:"api_url" jsonschema:"description=API URL of the GHES instance (e.g. https://ghes.example.com)"`
}

type File struct {
	Pattern string `json:"pattern"`
}

var (
	errUnsupportedConfigVersion = errors.New("pinact doesn't support this configuration format version. Maybe you need to update pinact")
	errAbandonedConfigVersion   = errors.New("this version was abandoned. Please update the schema version")
	errEmptyConfigVersion       = errors.New("schema version is required")
)

// validateSchemaVersion checks if the provided configuration schema version is supported.
// It validates against the current supported version (3) and provides helpful error
// messages for unsupported, abandoned, or missing versions.
//
// Parameters:
//   - v: schema version number to validate
//
// Returns an error if the version is not supported, nil if valid.
func validateSchemaVersion(v int) error {
	switch v {
	case 0:
		return slogerr.With(errEmptyConfigVersion, //nolint:wrapcheck
			"docs", "https://github.com/suzuki-shunsuke/pinact/blob/main/docs/codes/002.md",
		)
	case 2: //nolint:mnd
		return slogerr.With(errAbandonedConfigVersion, //nolint:wrapcheck
			"docs", "https://github.com/suzuki-shunsuke/pinact/blob/main/docs/codes/003.md",
		)
	case 3: //nolint:mnd
		return nil
	default:
		return slogerr.With(errUnsupportedConfigVersion, //nolint:wrapcheck
			"docs", "https://github.com/suzuki-shunsuke/pinact/blob/main/docs/codes/004.md",
		)
	}
}

// Init initializes and validates a File configuration.
// It validates the pattern field and ensures it's a valid glob pattern.
//
// Parameters:
//   - v: configuration schema version
//
// Returns an error if validation fails.
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

// Init initializes and validates an IgnoreAction configuration.
// It compiles the name and ref patterns as regular expressions.
//
// Parameters:
//   - v: configuration schema version
//
// Returns an error if initialization or validation fails.
func (ia *IgnoreAction) Init(v int) error {
	if err := ia.initName(); err != nil {
		return err
	}
	if err := ia.initRef(v); err != nil {
		return err
	}
	return nil
}

// Match checks if an action matches the ignore criteria.
// It evaluates both name and ref patterns against the provided values.
//
// Parameters:
//   - name: action name to match against
//   - ref: action reference to match against
//   - version: configuration schema version
//
// Returns true if the action should be ignored, false otherwise, or an error if matching fails.
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

// initName compiles the name pattern as a regular expression.
// It validates that a name pattern is provided and can be compiled.
//
// Returns an error if the name is empty or the regex compilation fails.
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

// initRef compiles the ref pattern as a regular expression.
// It validates that a ref pattern is provided and can be compiled.
//
// Parameters:
//   - v: configuration schema version
//
// Returns an error if the ref is empty or the regex compilation fails.
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

// matchName checks if the provided name matches the compiled name pattern.
// It performs exact string matching using the regular expression.
//
// Parameters:
//   - name: action name to match
//   - version: configuration schema version
//
// Returns true if the name matches exactly, false otherwise, or an error if validation fails.
func (ia *IgnoreAction) matchName(name string, version int) (bool, error) {
	if err := validateSchemaVersion(version); err != nil {
		return false, err
	}
	return ia.nameRegexp.FindString(name) == name, nil
}

// matchRef checks if the provided ref matches the compiled ref pattern.
// It performs exact string matching using the regular expression.
//
// Parameters:
//   - ref: action reference to match
//   - version: configuration schema version
//
// Returns true if the ref matches exactly, false otherwise, or an error if validation fails.
func (ia *IgnoreAction) matchRef(ref string, version int) (bool, error) {
	if err := validateSchemaVersion(version); err != nil {
		return false, err
	}
	return ia.refRegexp.FindString(ref) == ref, nil
}

// IsEnabled checks if GHES is enabled.
// GHES is enabled if the APIURL is set.
func (g *GHES) IsEnabled() bool {
	return g != nil && g.APIURL != ""
}

const githubAPIURL = "https://api.github.com"

// GHESFromEnv creates a GHES configuration from environment variables.
//
// Resolution priority for API URL:
//  1. PINACT_GHES_API_URL - if set, it is used (and GITHUB_API_URL is ignored)
//  2. GITHUB_API_URL - used as fallback if it's not https://api.github.com
//
// Returns nil if no GHES API URL is found.
func GHESFromEnv() *GHES {
	apiURL := os.Getenv("PINACT_GHES_API_URL")
	if apiURL == "" {
		githubURL := os.Getenv("GITHUB_API_URL")
		if githubURL == "" || githubURL == githubAPIURL {
			return nil
		}
		apiURL = githubURL
	}

	return &GHES{
		APIURL: apiURL,
	}
}

func (g *GHES) Validate() error {
	if g == nil {
		return nil
	}
	if g.APIURL == "" {
		return errors.New("GHES api_url is required")
	}
	return nil
}

// MergeFromEnv merges environment variable values into GHES configuration.
// If api_url is empty in the config, it fills it from environment variables.
func (g *GHES) MergeFromEnv() {
	if g == nil {
		return
	}
	if g.APIURL == "" {
		g.APIURL = os.Getenv("PINACT_GHES_API_URL")
		if g.APIURL == "" {
			githubURL := os.Getenv("GITHUB_API_URL")
			if githubURL != githubAPIURL {
				g.APIURL = githubURL
			}
		}
	}
}

// getConfigPath searches for a pinact configuration file in standard locations.
// It checks for .pinact.yaml, .github/pinact.yaml, .pinact.yml, and .github/pinact.yml
// in order of preference.
// Returns the path to the first found configuration file, empty string if none found, or an error.
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

// NewFinder creates a new configuration file finder.
// It initializes a Finder with the provided filesystem interface.
//
// Parameters:
//   - fs: filesystem interface for file operations
//
// Returns a pointer to the configured Finder.
func NewFinder(fs afero.Fs) *Finder {
	return &Finder{fs: fs}
}

// Find locates the configuration file path to use.
// If a specific path is provided, it returns that path.
// Otherwise, it searches for configuration files in standard locations.
//
// Parameters:
//   - configFilePath: explicit configuration file path or empty string
//
// Returns the configuration file path to use or an error if search fails.
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

// NewReader creates a new configuration file reader.
// It initializes a Reader with the provided filesystem interface.
//
// Parameters:
//   - fs: filesystem interface for file operations
//
// Returns a pointer to the configured Reader.
func NewReader(fs afero.Fs) *Reader {
	return &Reader{fs: fs}
}

// Read loads and parses a configuration file.
// It reads the YAML file, validates the schema version, and initializes
// all configuration components including files and ignore actions.
//
// Parameters:
//   - cfg: Config struct to populate with parsed data
//   - configFilePath: path to the configuration file to read
//
// Returns an error if reading, parsing, or validation fails.
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
	return cfg.Init()
}

// Init initializes and validates the configuration.
// It validates the schema version and initializes all configuration components.
//
// Returns an error if validation or initialization fails.
func (cfg *Config) Init() error {
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
