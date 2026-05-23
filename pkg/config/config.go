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
	"path/filepath"
	"regexp"
	"runtime"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
	"gopkg.in/yaml.v3"
)

const (
	pathPinactYaml       = ".pinact.yaml"
	pathPinactYml        = ".pinact.yml"
	pathGitHubPinactYaml = ".github/pinact.yaml"
	pathGitHubPinactYml  = ".github/pinact.yml"
)

type Config struct {
	Version       int             `json:"version,omitempty" jsonschema:"enum=2,enum=3"`
	Files         []*File         `json:"files,omitempty" jsonschema:"description=Target files. If files are passed via positional command line arguments, this is ignored"`
	IgnoreActions []*IgnoreAction `json:"ignore_actions,omitempty" yaml:"ignore_actions" jsonschema:"description=Actions and reusable workflows that pinact ignores. For new configurations consider using 'rules' with 'ignore: true' for more flexibility"`
	GHES          *GHES           `json:"ghes,omitempty" yaml:"ghes" jsonschema:"description=GitHub Enterprise Server configuration"`
	Separator     string          `json:"separator,omitempty" jsonschema:"description=Separator between version and tag comment. Default is ' # '"`
	MinAge        *MinAge         `json:"min_age,omitzero" yaml:"min_age" jsonschema:"description=Default min-age settings. value is the threshold in days; always opts every run into the passive audit. rules can override value per action"`
	Rules         []*Rule         `json:"rules,omitempty" jsonschema:"description=Per-action setting overrides. Later matching rules override earlier ones at the field level"`
}

// MinAge controls both the threshold and whether the passive audit auto-runs.
//
// Value is the default min-age threshold in days. It is used as the update
// target gate when -update is set, and as the cutoff for the passive audit
// when the audit runs. rules[].min_age and the -min-age CLI flag can override
// Value per action / per run.
//
// Always opts every `pinact run` into the passive audit even without the
// -verify-min-age CLI flag. Default false so config.min_age alone does not
// add a GetCommit call per pinned action on every run.
//
// Value and Always are pointers so we can distinguish "unset" from the zero
// value when merging a project config on top of a global config: a project
// that omits min_age.always must not clobber a global true with a false.
type MinAge struct {
	Value  *int  `json:"value,omitempty" jsonschema:"description=Default min-age threshold in days"`
	Always *bool `json:"always,omitempty" jsonschema:"description=When true every run performs the passive min-age audit. Default false"`
}

type GHES struct {
	APIURL   string `json:"api_url,omitempty" yaml:"api_url" jsonschema:"description=API URL of the GHES instance (e.g. https://ghes.example.com)"`
	Fallback bool   `json:"fallback,omitempty" yaml:"fallback" jsonschema:"description=Whether to fallback to github.com when a repository is not found on GHES. Default is false"`
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
		return slogerr.With( //nolint:wrapcheck
			errEmptyConfigVersion,
			"docs", "https://github.com/suzuki-shunsuke/pinact/blob/main/docs/codes/002.md",
		)
	case 2: //nolint:mnd
		return slogerr.With( //nolint:wrapcheck
			errAbandonedConfigVersion,
			"docs", "https://github.com/suzuki-shunsuke/pinact/blob/main/docs/codes/003.md",
		)
	case 3: //nolint:mnd
		return nil
	default:
		return slogerr.With( //nolint:wrapcheck
			errUnsupportedConfigVersion,
			"docs", "https://github.com/suzuki-shunsuke/pinact/blob/main/docs/codes/004.md",
		)
	}
}

// Init validates the file pattern. The schema version is validated by
// Config.Init before this method runs, so it is not re-checked here.
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
	Name       string `json:"name"`
	Ref        string `json:"ref,omitempty"`
	nameRegexp *regexp.Regexp
	refRegexp  *regexp.Regexp
}

// Init compiles the name and ref patterns as regular expressions. The schema
// version is validated by Config.Init before this method runs.
func (ia *IgnoreAction) Init() error {
	if err := ia.initName(); err != nil {
		return err
	}
	return ia.initRef()
}

// Match reports whether name and ref match this ignore entry. Both must match.
func (ia *IgnoreAction) Match(name, ref string) bool {
	if ia.nameRegexp.FindString(name) != name {
		return false
	}
	return ia.refRegexp.FindString(ref) == ref
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

func (ia *IgnoreAction) initRef() error {
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

// Rule overrides per-action settings (ignore, min_age) for actions that match
// any of its conditions. Multiple matching rules are merged at the field level
// in declaration order: later rules override earlier ones, but only for fields
// they explicitly set.
type Rule struct {
	Ignore     *bool        `json:"ignore,omitempty" jsonschema:"description=If true pinact skips pin/update/error for the matched action"`
	MinAge     *int         `json:"min_age,omitempty" yaml:"min_age" jsonschema:"description=Override the min-age threshold (in days) for the matched action. 0 disables the check for the action"`
	Conditions []*Condition `json:"conditions,omitempty" jsonschema:"description=Match conditions. The rule matches if any condition evaluates to true"`
}

// Condition is one of the expressions in a rule. The rule matches when at
// least one of its conditions evaluates to true.
type Condition struct {
	Expr    string      `json:"expr" jsonschema:"description=A boolean expression. See https://expr-lang.org/docs/language-definition"`
	program *vm.Program // cached compiled program, populated by Init
}

// MatchInput holds the variables exposed to expr expressions when evaluating
// rule conditions.
type MatchInput struct {
	ActionName         string
	ActionRepoOwner    string
	ActionRepoName     string
	ActionRepoFullName string
	ActionVersion      string
	VersionComment     string
}

// Resolved is the merged result of all rules that matched a given action.
// MinAge is a pointer because nil means "no rule overrode min_age", which is
// distinct from a rule explicitly setting min_age to 0 (which disables the
// check for that action).
type Resolved struct {
	Ignore bool
	MinAge *int
}

var (
	errEmptyConditions = errors.New("rule must have at least one condition")
	errEmptyExpr       = errors.New("expr is required")
	errMustBeBoolean   = errors.New("expr must evaluate to a boolean")
)

// Init validates and compiles a Rule. Conditions are compiled once and cached
// on the Condition struct so evaluation does not pay the compile cost per
// action. The schema version is validated by Config.Init before this method
// runs.
func (r *Rule) Init() error {
	if len(r.Conditions) == 0 {
		return errEmptyConditions
	}
	for i, c := range r.Conditions {
		if err := c.Init(); err != nil {
			return fmt.Errorf("initialize conditions[%d]: %w", i, err)
		}
	}
	return nil
}

// Init compiles the expression and caches the resulting program. Compile-time
// errors (syntax errors, references to undefined variables, non-boolean
// expressions) are surfaced as config errors so they fail fast at startup.
func (c *Condition) Init() error {
	if c.Expr == "" {
		return errEmptyExpr
	}
	prog, err := expr.Compile(c.Expr, expr.AsBool(), expr.Env(MatchInput{}))
	if err != nil {
		return fmt.Errorf("compile expr: %w", err)
	}
	c.program = prog
	return nil
}

// Match reports whether any of the rule's conditions evaluates to true for the
// given input. Errors propagate up so the caller can decide whether to skip the
// rule or abort.
func (r *Rule) Match(input *MatchInput) (bool, error) {
	for i, c := range r.Conditions {
		out, err := expr.Run(c.program, input)
		if err != nil {
			return false, fmt.Errorf("evaluate conditions[%d]: %w", i, err)
		}
		b, ok := out.(bool)
		if !ok {
			return false, errMustBeBoolean
		}
		if b {
			return true, nil
		}
	}
	return false, nil
}

// ResolveRules evaluates every rule against input and merges the matching
// rules. Fields are merged independently: a rule that only sets MinAge leaves
// a previously matched rule's Ignore untouched. Later matching rules override
// earlier ones for the fields they set.
func (cfg *Config) ResolveRules(input *MatchInput) (*Resolved, error) {
	res := &Resolved{}
	for i, r := range cfg.Rules {
		matched, err := r.Match(input)
		if err != nil {
			return nil, fmt.Errorf("evaluate rules[%d]: %w", i, err)
		}
		if !matched {
			continue
		}
		if r.Ignore != nil {
			res.Ignore = *r.Ignore
		}
		if r.MinAge != nil {
			res.MinAge = r.MinAge
		}
	}
	return res, nil
}

// IsEnabled checks if GHES is enabled.
// GHES is enabled if the APIURL is set.
func (g *GHES) IsEnabled() bool {
	return g != nil && g.APIURL != ""
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

// findProjectConfigPath searches for a project-level pinact configuration file
// in standard locations. It checks .pinact.yaml, .github/pinact.yaml,
// .pinact.yml, and .github/pinact.yml in order of preference. Returns the path
// to the first match, "" if none found, or an error.
func findProjectConfigPath(fs afero.Fs) (string, error) {
	for _, p := range []string{pathPinactYaml, pathGitHubPinactYaml, pathPinactYml, pathGitHubPinactYml} {
		f, err := afero.Exists(fs, p)
		if err != nil {
			return "", fmt.Errorf("check if %s exists: %w", p, err)
		}
		if f {
			return p, nil
		}
	}
	return "", nil
}

// findGlobalConfigPath returns the path of the user-wide pinact config file
// if it exists, or "". The path itself is resolved from PINACT_GLOBAL_CONFIG
// or the platform-native location; see resolveGlobalConfigPath.
func findGlobalConfigPath(fs afero.Fs) (string, error) {
	globalPath := resolveGlobalConfigPath(runtime.GOOS, os.Getenv, getHomeDir())
	if globalPath == "" {
		return "", nil
	}
	f, err := afero.Exists(fs, globalPath)
	if err != nil {
		return "", fmt.Errorf("check if %s exists: %w", globalPath, err)
	}
	if f {
		return globalPath, nil
	}
	return "", nil
}

// GlobalConfigPath returns the absolute path of the pinact global config file
// for the current platform, or "" if it cannot be resolved (e.g. APPDATA is
// unset on Windows, or the user has no home directory). Callers that need to
// surface a clear error to the user should check for the empty return.
func GlobalConfigPath() string {
	return resolveGlobalConfigPath(runtime.GOOS, os.Getenv, getHomeDir())
}

// resolveGlobalConfigPath returns the path of the pinact global config file,
// or "" if it cannot be resolved. Precedence:
//
//  1. PINACT_GLOBAL_CONFIG env var (used verbatim; absolute or relative).
//  2. Platform-native location:
//     - Linux / macOS: $XDG_CONFIG_HOME/pinact/pinact.yaml when
//     XDG_CONFIG_HOME is set, otherwise <home>/.config/pinact/pinact.yaml.
//     macOS deliberately uses the XDG layout rather than
//     ~/Library/Application Support to avoid the space in the path and to
//     match what most developer tooling expects.
//     - Windows: %APPDATA%\pinact\pinact.yaml. APPDATA is the Roaming
//     AppData folder, the standard location for user-specific config that
//     should follow the user across machines.
//
// PINACT_GLOBAL_CONFIG is not subject to ~/$VAR expansion; callers can use
// shell expansion at the point of invocation if they need it.
func resolveGlobalConfigPath(goos string, getEnv func(string) string, homeDir string) string {
	if p := getEnv("PINACT_GLOBAL_CONFIG"); p != "" {
		return p
	}
	const windows = "windows"
	if goos == windows {
		appData := getEnv("APPDATA")
		if appData == "" {
			return ""
		}
		return filepath.Join(appData, "pinact", "pinact.yaml")
	}
	if xdg := getEnv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "pinact", "pinact.yaml")
	}
	if homeDir == "" {
		return ""
	}
	return filepath.Join(homeDir, ".config", "pinact", "pinact.yaml")
}

func getHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

type Finder struct {
	fs afero.Fs
}

// NewFinder creates a new configuration file finder.
func NewFinder(fs afero.Fs) *Finder {
	return &Finder{fs: fs}
}

// Find locates the project-level configuration file path.
// If a specific path is provided (e.g. via -c or PINACT_CONFIG), it returns
// that path verbatim. Otherwise, it searches for configuration files in
// standard locations. Returns "" if no project config is found; callers that
// also want to consider a global config should call FindGlobal separately and
// merge with mergeConfig.
func (f *Finder) Find(configFilePath string) (string, error) {
	if configFilePath != "" {
		return configFilePath, nil
	}
	p, err := findProjectConfigPath(f.fs)
	if err != nil {
		return "", err
	}
	return p, nil
}

// FindGlobal returns the user-wide pinact config file path if the file exists,
// or "". The path itself is resolved from PINACT_GLOBAL_CONFIG when set, or
// otherwise from the platform-native location (see resolveGlobalConfigPath).
func (f *Finder) FindGlobal() (string, error) {
	return findGlobalConfigPath(f.fs)
}

type Reader struct {
	fs afero.Fs
}

// NewReader creates a new configuration file reader.
func NewReader(fs afero.Fs) *Reader {
	return &Reader{fs: fs}
}

// Read loads and parses a single configuration file.
// It reads the YAML file, validates the schema version, and initializes
// all configuration components including files and ignore actions.
// Callers that want global + project merge should use ReadAndMerge instead.
func (r *Reader) Read(cfg *Config, configFilePath string) error {
	if configFilePath == "" {
		return nil
	}
	if err := r.decode(cfg, configFilePath); err != nil {
		return err
	}
	return cfg.Init()
}

// ReadAndMerge loads project and/or global configs and merges them into cfg.
// Either path may be empty (caller has none of that kind). When both are
// present, the schema version of each is validated independently with the
// source path attached to any error, and then mergeConfig combines them
// (project wins per field, lists are concatenated; see mergeConfig).
//
// The resulting cfg is then Init'd so rules / files / ignore_actions are
// compiled. If neither path is given, cfg is left untouched.
func (r *Reader) ReadAndMerge(cfg *Config, projectPath, globalPath string) error {
	var project, global *Config
	if globalPath != "" {
		global = &Config{}
		if err := r.decode(global, globalPath); err != nil {
			return fmt.Errorf("read global configuration file %s: %w", globalPath, err)
		}
		if err := validateSchemaVersion(global.Version); err != nil {
			return fmt.Errorf("validate global configuration file %s: %w", globalPath, err)
		}
	}
	if projectPath != "" {
		project = &Config{}
		if err := r.decode(project, projectPath); err != nil {
			return fmt.Errorf("read project configuration file %s: %w", projectPath, err)
		}
		if err := validateSchemaVersion(project.Version); err != nil {
			return fmt.Errorf("validate project configuration file %s: %w", projectPath, err)
		}
	}
	merged := mergeConfig(global, project)
	if merged == nil {
		return nil
	}
	*cfg = *merged
	return cfg.Init()
}

func (r *Reader) decode(cfg *Config, configFilePath string) error {
	f, err := r.fs.Open(configFilePath)
	if err != nil {
		return fmt.Errorf("open a configuration file: %w", err)
	}
	defer f.Close()
	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return fmt.Errorf("decode a configuration file as YAML: %w", err)
	}
	return nil
}

// mergeConfig combines a global config with a project config so machine-wide
// defaults survive when a project also ships its own config. Project wins
// per field for scalars and the MinAge / GHES blocks; rules and ignore_actions
// are concatenated as [global..., project...] so later (project) entries take
// effect first under existing rule-resolution semantics.
//
// Either argument may be nil; mergeConfig returns the other unchanged.
func mergeConfig(global, project *Config) *Config {
	if global == nil {
		return project
	}
	if project == nil {
		return global
	}
	out := *project
	if out.Version == 0 {
		out.Version = global.Version
	}
	if len(out.Files) == 0 {
		out.Files = global.Files
	}
	if out.Separator == "" {
		out.Separator = global.Separator
	}
	if out.GHES == nil {
		out.GHES = global.GHES
	}
	out.IgnoreActions = append(append([]*IgnoreAction(nil), global.IgnoreActions...), project.IgnoreActions...)
	out.Rules = append(append([]*Rule(nil), global.Rules...), project.Rules...)
	out.MinAge = mergeMinAge(global.MinAge, project.MinAge)
	return &out
}

// mergeMinAge merges MinAge field-by-field. value and always are merged
// independently: a project-level value override does not clobber a global
// always:true (the typical safety scenario for global config).
func mergeMinAge(global, project *MinAge) *MinAge {
	if global == nil {
		return project
	}
	if project == nil {
		return global
	}
	out := MinAge{
		Value:  global.Value,
		Always: global.Always,
	}
	if project.Value != nil {
		out.Value = project.Value
	}
	if project.Always != nil {
		out.Always = project.Always
	}
	return &out
}

// Init initializes and validates the configuration.
// It validates the schema version and initializes all configuration components.
func (cfg *Config) Init() error {
	if err := validateSchemaVersion(cfg.Version); err != nil {
		return err
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
	for i, r := range cfg.Rules {
		if err := r.Init(); err != nil {
			return fmt.Errorf("initialize rules[%d]: %w", i, err)
		}
	}
	return nil
}
