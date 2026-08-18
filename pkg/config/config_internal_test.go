package config

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

func Test_validateSchemaVersion(t *testing.T) {
	t.Parallel()
	data := []struct {
		name    string
		version int
		wantErr bool
	}{
		{name: "version 0 - empty", version: 0, wantErr: true},
		{name: "version 2 - abandoned", version: 2, wantErr: true},
		{name: "version 3 - valid", version: 3, wantErr: false},
		{name: "version 4 - unsupported", version: 4, wantErr: true},
		{name: "version 99 - unsupported", version: 99, wantErr: true},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			err := validateSchemaVersion(d.version)
			if d.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !d.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestFile_Init(t *testing.T) {
	t.Parallel()
	data := []struct {
		name    string
		file    *File
		wantErr bool
	}{
		{name: "valid pattern", file: &File{Pattern: "*.yaml"}, wantErr: false},
		{name: "empty pattern", file: &File{Pattern: ""}, wantErr: true},
		{name: "invalid glob pattern", file: &File{Pattern: "[invalid"}, wantErr: true},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			err := d.file.Init()
			if d.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !d.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestIgnoreAction_Init(t *testing.T) {
	t.Parallel()
	data := []struct {
		name    string
		ia      *IgnoreAction
		wantErr bool
	}{
		{name: "valid", ia: &IgnoreAction{Name: "actions/checkout", Ref: "v4"}, wantErr: false},
		{name: "empty name", ia: &IgnoreAction{Name: "", Ref: "v4"}, wantErr: true},
		{name: "empty ref", ia: &IgnoreAction{Name: "actions/checkout", Ref: ""}, wantErr: true},
		{name: "invalid name regex", ia: &IgnoreAction{Name: "[invalid", Ref: "v4"}, wantErr: true},
		{name: "invalid ref regex", ia: &IgnoreAction{Name: "actions/checkout", Ref: "[invalid"}, wantErr: true},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			err := d.ia.Init()
			if d.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !d.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGHES_IsEnabled(t *testing.T) {
	t.Parallel()
	data := []struct {
		name string
		ghes *GHES
		exp  bool
	}{
		{name: "nil", ghes: nil, exp: false},
		{name: "empty api url", ghes: &GHES{APIURL: ""}, exp: false},
		{name: "with api url", ghes: &GHES{APIURL: "https://ghes.example.com"}, exp: true},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			if got := d.ghes.IsEnabled(); got != d.exp {
				t.Errorf("wanted %v, got %v", d.exp, got)
			}
		})
	}
}

func TestGHES_Validate(t *testing.T) {
	t.Parallel()
	data := []struct {
		name    string
		ghes    *GHES
		wantErr bool
	}{
		{name: "nil", ghes: nil, wantErr: false},
		{name: "empty api url", ghes: &GHES{APIURL: ""}, wantErr: true},
		{name: "with api url", ghes: &GHES{APIURL: "https://ghes.example.com"}, wantErr: false},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			err := d.ghes.Validate()
			if d.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !d.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestFinder_Find(t *testing.T) {
	t.Parallel()
	t.Run("explicit path", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		finder := NewFinder(fs)
		got, err := finder.Find("/custom/path.yaml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "/custom/path.yaml" {
			t.Errorf("wanted %q, got %q", "/custom/path.yaml", got)
		}
	})

	t.Run("search default paths", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		if err := afero.WriteFile(fs, ".github/pinact.yaml", []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		finder := NewFinder(fs)
		got, err := finder.Find("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != ".github/pinact.yaml" {
			t.Errorf("wanted %q, got %q", ".github/pinact.yaml", got)
		}
	})
}

func Test_resolveGlobalConfigPath(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name    string
		goos    string
		env     map[string]string
		homeDir string
		want    string
	}{
		{
			name:    "linux with XDG_CONFIG_HOME",
			goos:    "linux",
			env:     map[string]string{"XDG_CONFIG_HOME": "/xdg"},
			homeDir: "/home/user",
			want:    "/xdg/pinact/pinact.yaml",
		},
		{
			name:    "linux without XDG falls back to ~/.config",
			goos:    "linux",
			env:     map[string]string{},
			homeDir: "/home/user",
			want:    "/home/user/.config/pinact/pinact.yaml",
		},
		{
			name:    "macOS without XDG also falls back to ~/.config",
			goos:    "darwin",
			env:     map[string]string{},
			homeDir: "/Users/user",
			want:    "/Users/user/.config/pinact/pinact.yaml",
		},
		{
			name:    "macOS XDG is honored",
			goos:    "darwin",
			env:     map[string]string{"XDG_CONFIG_HOME": "/Users/user/.cfg"},
			homeDir: "/Users/user",
			want:    "/Users/user/.cfg/pinact/pinact.yaml",
		},
		{
			name: "windows uses APPDATA",
			goos: "windows",
			env:  map[string]string{"APPDATA": `C:\Users\user\AppData\Roaming`},
			// homeDir is ignored on Windows; APPDATA wins.
			homeDir: `C:\Users\user`,
			// Use filepath.Join in the expected value too so the test passes
			// on both POSIX (`/` separator) and Windows (`\` separator) hosts.
			want: filepath.Join(`C:\Users\user\AppData\Roaming`, "pinact", "pinact.yaml"),
		},
		{
			name:    "windows without APPDATA returns empty",
			goos:    "windows",
			env:     map[string]string{},
			homeDir: `C:\Users\user`,
			want:    "",
		},
		{
			name:    "unix without home dir returns empty",
			goos:    "linux",
			env:     map[string]string{},
			homeDir: "",
			want:    "",
		},
		{
			name:    "PINACT_GLOBAL_CONFIG wins over linux XDG path",
			goos:    "linux",
			env:     map[string]string{"PINACT_GLOBAL_CONFIG": "/tmp/custom.yaml", "XDG_CONFIG_HOME": "/xdg"},
			homeDir: "/home/user",
			want:    "/tmp/custom.yaml",
		},
		{
			name:    "PINACT_GLOBAL_CONFIG wins over macOS default",
			goos:    "darwin",
			env:     map[string]string{"PINACT_GLOBAL_CONFIG": "/Users/user/dotfiles/pinact.yaml"},
			homeDir: "/Users/user",
			want:    "/Users/user/dotfiles/pinact.yaml",
		},
		{
			name:    "PINACT_GLOBAL_CONFIG wins over Windows APPDATA",
			goos:    "windows",
			env:     map[string]string{"PINACT_GLOBAL_CONFIG": `D:\config\pinact.yaml`, "APPDATA": `C:\Users\user\AppData\Roaming`},
			homeDir: `C:\Users\user`,
			want:    `D:\config\pinact.yaml`,
		},
		{
			name:    "PINACT_GLOBAL_CONFIG empty string falls back to OS default",
			goos:    "linux",
			env:     map[string]string{"PINACT_GLOBAL_CONFIG": "", "XDG_CONFIG_HOME": "/xdg"},
			homeDir: "/home/user",
			want:    "/xdg/pinact/pinact.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			getEnv := func(k string) string { return tt.env[k] }
			got := resolveGlobalConfigPath(tt.goos, getEnv, tt.homeDir)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReader_Read(t *testing.T) { //nolint:cyclop,funlen
	t.Parallel()
	t.Run("empty path", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		reader := NewReader(fs)
		cfg := &Config{}
		if err := reader.Read(cfg, ""); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("valid config", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		content := `version: 3
files:
  - pattern: "*.yaml"
`
		if err := afero.WriteFile(fs, ".pinact.yaml", []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		reader := NewReader(fs)
		cfg := &Config{}
		if err := reader.Read(cfg, ".pinact.yaml"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Version != 3 {
			t.Errorf("Version: wanted 3, got %d", cfg.Version)
		}
		if len(cfg.Files) != 1 {
			t.Errorf("Files length: wanted 1, got %d", len(cfg.Files))
		}
	})

	t.Run("file not found", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		reader := NewReader(fs)
		cfg := &Config{}
		if err := reader.Read(cfg, "nonexistent.yaml"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		if err := afero.WriteFile(fs, ".pinact.yaml", []byte("invalid: yaml: content:"), 0o644); err != nil {
			t.Fatal(err)
		}
		reader := NewReader(fs)
		cfg := &Config{}
		if err := reader.Read(cfg, ".pinact.yaml"); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("min_age value and always populate", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		content := `version: 3
min_age:
  value: 60
  always: true
`
		if err := afero.WriteFile(fs, ".pinact.yaml", []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		reader := NewReader(fs)
		cfg := &Config{}
		if err := reader.Read(cfg, ".pinact.yaml"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.MinAge.Value == nil || *cfg.MinAge.Value != 60 {
			t.Errorf("MinAge.Value: wanted 60, got %v", cfg.MinAge.Value)
		}
		if cfg.MinAge.Always == nil || !*cfg.MinAge.Always {
			t.Errorf("MinAge.Always: wanted true, got %v", cfg.MinAge.Always)
		}
	})
}

func TestConfig_Init(t *testing.T) {
	t.Parallel()
	t.Run("valid config", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Version: 3,
			Files:   []*File{{Pattern: "*.yaml"}},
			IgnoreActions: []*IgnoreAction{
				{Name: "actions/checkout", Ref: "v4"},
			},
		}
		if err := cfg.Init(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid version", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{Version: 0}
		if err := cfg.Init(); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("invalid file", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Version: 3,
			Files:   []*File{{Pattern: ""}},
		}
		if err := cfg.Init(); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("invalid ignore action", func(t *testing.T) {
		t.Parallel()
		cfg := &Config{
			Version:       3,
			IgnoreActions: []*IgnoreAction{{Name: "", Ref: "v4"}},
		}
		if err := cfg.Init(); err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func Test_findProjectConfigPath(t *testing.T) {
	t.Parallel()
	data := []struct {
		name  string
		paths []string
		exp   string
	}{
		{
			name:  "no config",
			paths: []string{},
			exp:   "",
		},
		{
			name:  "primary",
			paths: []string{".pinact.yaml"},
			exp:   ".pinact.yaml",
		},
		{
			name:  "another",
			paths: []string{".github/pinact.yaml"},
			exp:   ".github/pinact.yaml",
		},
		{
			name:  "both primary and others",
			paths: []string{".pinact.yaml", ".github/pinact.yaml"},
			exp:   ".pinact.yaml",
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			for _, path := range d.paths {
				if err := afero.WriteFile(fs, path, []byte(""), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := findProjectConfigPath(fs)
			if err != nil {
				t.Fatal(err)
			}
			if got != d.exp {
				t.Fatalf(`wanted %s, got %s`, d.exp, got)
			}
		})
	}
}

func TestRule_Init(t *testing.T) {
	t.Parallel()
	data := []struct {
		name    string
		rule    *Rule
		wantErr bool
	}{
		{
			name: "valid",
			rule: &Rule{
				Conditions: []*Condition{{Expr: `ActionName == "actions/checkout"`}},
			},
			wantErr: false,
		},
		{
			name:    "empty conditions rejected",
			rule:    &Rule{},
			wantErr: true,
		},
		{
			name: "empty expr rejected",
			rule: &Rule{
				Conditions: []*Condition{{Expr: ""}},
			},
			wantErr: true,
		},
		{
			name: "syntax error rejected at init",
			rule: &Rule{
				Conditions: []*Condition{{Expr: `ActionName ==`}},
			},
			wantErr: true,
		},
		{
			name: "non-boolean expr rejected at init",
			rule: &Rule{
				Conditions: []*Condition{{Expr: `ActionName`}},
			},
			wantErr: true,
		},
		{
			name: "undefined variable rejected at init",
			rule: &Rule{
				Conditions: []*Condition{{Expr: `Unknown == "x"`}},
			},
			wantErr: true,
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			err := d.rule.Init()
			if d.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !d.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestConfig_ResolveRules(t *testing.T) { //nolint:funlen
	t.Parallel()
	input := &MatchInput{
		ActionName:         "actions/checkout",
		ActionRepoOwner:    "actions",
		ActionRepoName:     "checkout",
		ActionRepoFullName: "actions/checkout",
		ActionVersion:      "v4",
		VersionComment:     "",
	}

	tests := []struct {
		name          string
		rules         []*Rule
		wantIgnore    bool
		wantMinAge    *int
		wantKeepMajor *bool
	}{
		{
			name:       "no rules",
			rules:      nil,
			wantIgnore: false,
			wantMinAge: nil,
		},
		{
			name: "single matching rule sets ignore",
			rules: []*Rule{
				{
					Ignore:     new(true),
					Conditions: []*Condition{{Expr: `ActionName == "actions/checkout"`}},
				},
			},
			wantIgnore: true,
			wantMinAge: nil,
		},
		{
			name: "non-matching rule has no effect",
			rules: []*Rule{
				{
					Ignore:     new(true),
					Conditions: []*Condition{{Expr: `ActionName == "octocat/hello-world"`}},
				},
			},
			wantIgnore: false,
			wantMinAge: nil,
		},
		{
			name: "OR semantics across conditions",
			rules: []*Rule{
				{
					Ignore: new(true),
					Conditions: []*Condition{
						{Expr: `ActionName == "octocat/hello-world"`},
						{Expr: `ActionRepoOwner == "actions"`},
					},
				},
			},
			wantIgnore: true,
			wantMinAge: nil,
		},
		{
			name: "later rule overrides only the field it sets",
			rules: []*Rule{
				{
					Ignore:     new(true),
					Conditions: []*Condition{{Expr: `ActionRepoOwner == "actions"`}},
				},
				{
					MinAge:     new(0),
					Conditions: []*Condition{{Expr: `ActionName == "actions/checkout"`}},
				},
			},
			wantIgnore: true,
			wantMinAge: new(0),
		},
		{
			name: "later matching rule overrides ignore",
			rules: []*Rule{
				{
					Ignore:     new(true),
					Conditions: []*Condition{{Expr: `ActionRepoOwner == "actions"`}},
				},
				{
					Ignore:     new(false),
					Conditions: []*Condition{{Expr: `ActionName == "actions/checkout"`}},
				},
			},
			wantIgnore: false,
			wantMinAge: nil,
		},
		{
			name: "min_age 0 from rule disables check",
			rules: []*Rule{
				{
					MinAge:     new(0),
					Conditions: []*Condition{{Expr: `ActionVersion == "v4"`}},
				},
			},
			wantIgnore: false,
			wantMinAge: new(0),
		},
		{
			name: "rule sets keep_major true",
			rules: []*Rule{
				{
					KeepMajor:  new(true),
					Conditions: []*Condition{{Expr: `ActionRepoOwner == "actions"`}},
				},
			},
			wantIgnore:    false,
			wantMinAge:    nil,
			wantKeepMajor: new(true),
		},
		{
			name: "rule sets keep_major false (explicit opt-out)",
			rules: []*Rule{
				{
					KeepMajor:  new(false),
					Conditions: []*Condition{{Expr: `ActionName == "actions/checkout"`}},
				},
			},
			wantIgnore:    false,
			wantMinAge:    nil,
			wantKeepMajor: new(false),
		},
		{
			name: "later matching rule overrides keep_major",
			rules: []*Rule{
				{
					KeepMajor:  new(true),
					Conditions: []*Condition{{Expr: `ActionRepoOwner == "actions"`}},
				},
				{
					KeepMajor:  new(false),
					Conditions: []*Condition{{Expr: `ActionName == "actions/checkout"`}},
				},
			},
			wantIgnore:    false,
			wantMinAge:    nil,
			wantKeepMajor: new(false),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &Config{Version: 3, Rules: tt.rules}
			if err := cfg.Init(); err != nil {
				t.Fatalf("Init: %v", err)
			}
			got, err := cfg.ResolveRules(input)
			if err != nil {
				t.Fatalf("ResolveRules: %v", err)
			}
			if got.Ignore != tt.wantIgnore {
				t.Errorf("Ignore: got %v, want %v", got.Ignore, tt.wantIgnore)
			}
			switch {
			case got.MinAge == nil && tt.wantMinAge == nil:
				// ok
			case got.MinAge == nil || tt.wantMinAge == nil:
				t.Errorf("MinAge: got %v, want %v", got.MinAge, tt.wantMinAge)
			case *got.MinAge != *tt.wantMinAge:
				t.Errorf("MinAge: got %d, want %d", *got.MinAge, *tt.wantMinAge)
			}
			switch {
			case got.KeepMajor == nil && tt.wantKeepMajor == nil:
				// ok
			case got.KeepMajor == nil || tt.wantKeepMajor == nil:
				t.Errorf("KeepMajor: got %v, want %v", got.KeepMajor, tt.wantKeepMajor)
			case *got.KeepMajor != *tt.wantKeepMajor:
				t.Errorf("KeepMajor: got %v, want %v", *got.KeepMajor, *tt.wantKeepMajor)
			}
		})
	}
}

func TestMergeConfig(t *testing.T) { //nolint:funlen
	t.Parallel()

	ruleA := &Rule{MinAge: new(0), Conditions: []*Condition{{Expr: `true`}}}
	ruleB := &Rule{Ignore: new(true), Conditions: []*Condition{{Expr: `true`}}}
	ignoreA := &IgnoreAction{Name: "foo/.*", Ref: "main"}
	ignoreB := &IgnoreAction{Name: "bar/.*", Ref: "main"}

	tests := []struct {
		name    string
		global  *Config
		project *Config
		want    *Config
	}{
		{
			name:    "both nil returns nil",
			global:  nil,
			project: nil,
			want:    nil,
		},
		{
			name:    "global only",
			global:  &Config{Version: 3, Separator: " # "},
			project: nil,
			want:    &Config{Version: 3, Separator: " # "},
		},
		{
			name:    "project only",
			global:  nil,
			project: &Config{Version: 3, Separator: " # "},
			want:    &Config{Version: 3, Separator: " # "},
		},
		{
			name: "min_age value and always merged independently",
			global: &Config{
				Version: 3,
				MinAge:  &MinAge{Value: new(7), Always: new(true)},
			},
			project: &Config{
				Version: 3,
				MinAge:  &MinAge{Value: new(3)},
			},
			want: &Config{
				Version: 3,
				MinAge:  &MinAge{Value: new(3), Always: new(true)},
			},
		},
		{
			name: "project min_age.value 0 overrides global value",
			global: &Config{
				Version: 3,
				MinAge:  &MinAge{Value: new(7)},
			},
			project: &Config{
				Version: 3,
				MinAge:  &MinAge{Value: new(0)},
			},
			want: &Config{
				Version: 3,
				MinAge:  &MinAge{Value: new(0)},
			},
		},
		{
			name: "project min_age nil keeps global min_age",
			global: &Config{
				Version: 3,
				MinAge:  &MinAge{Value: new(7), Always: new(true)},
			},
			project: &Config{Version: 3},
			want: &Config{
				Version: 3,
				MinAge:  &MinAge{Value: new(7), Always: new(true)},
			},
		},
		{
			name:    "rules concatenated global then project",
			global:  &Config{Version: 3, Rules: []*Rule{ruleA}},
			project: &Config{Version: 3, Rules: []*Rule{ruleB}},
			want:    &Config{Version: 3, Rules: []*Rule{ruleA, ruleB}},
		},
		{
			name:    "ignore_actions concatenated global then project",
			global:  &Config{Version: 3, IgnoreActions: []*IgnoreAction{ignoreA}},
			project: &Config{Version: 3, IgnoreActions: []*IgnoreAction{ignoreB}},
			want:    &Config{Version: 3, IgnoreActions: []*IgnoreAction{ignoreA, ignoreB}},
		},
		{
			name:    "project files replaces global files",
			global:  &Config{Version: 3, Files: []*File{{Pattern: "global.yml"}}},
			project: &Config{Version: 3, Files: []*File{{Pattern: "project.yml"}}},
			want:    &Config{Version: 3, Files: []*File{{Pattern: "project.yml"}}},
		},
		{
			name:    "global files used when project files empty",
			global:  &Config{Version: 3, Files: []*File{{Pattern: "global.yml"}}},
			project: &Config{Version: 3},
			want:    &Config{Version: 3, Files: []*File{{Pattern: "global.yml"}}},
		},
		{
			name:    "project separator replaces global separator",
			global:  &Config{Version: 3, Separator: " # "},
			project: &Config{Version: 3, Separator: " #tag="},
			want:    &Config{Version: 3, Separator: " #tag="},
		},
		{
			name:    "global ghes carried when project has none",
			global:  &Config{Version: 3, GHES: &GHES{APIURL: "https://ghes.example.com"}},
			project: &Config{Version: 3},
			want:    &Config{Version: 3, GHES: &GHES{APIURL: "https://ghes.example.com"}},
		},
		{
			name:    "project ghes replaces global ghes",
			global:  &Config{Version: 3, GHES: &GHES{APIURL: "https://global.example.com"}},
			project: &Config{Version: 3, GHES: &GHES{APIURL: "https://project.example.com"}},
			want:    &Config{Version: 3, GHES: &GHES{APIURL: "https://project.example.com"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mergeConfig(tt.global, tt.project)
			assertConfigEqual(t, got, tt.want)
		})
	}
}

func assertConfigEqual(t *testing.T, got, want *Config) { //nolint:cyclop
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("config: got %v, want %v", got, want)
	}
	if got == nil {
		return
	}
	if got.Version != want.Version {
		t.Errorf("Version: got %d, want %d", got.Version, want.Version)
	}
	if got.Separator != want.Separator {
		t.Errorf("Separator: got %q, want %q", got.Separator, want.Separator)
	}
	if len(got.Files) != len(want.Files) {
		t.Fatalf("Files len: got %d, want %d", len(got.Files), len(want.Files))
	}
	for i := range got.Files {
		if got.Files[i].Pattern != want.Files[i].Pattern {
			t.Errorf("Files[%d].Pattern: got %q, want %q", i, got.Files[i].Pattern, want.Files[i].Pattern)
		}
	}
	if len(got.Rules) != len(want.Rules) {
		t.Fatalf("Rules len: got %d, want %d", len(got.Rules), len(want.Rules))
	}
	for i := range got.Rules {
		if got.Rules[i] != want.Rules[i] {
			t.Errorf("Rules[%d]: got %p, want %p", i, got.Rules[i], want.Rules[i])
		}
	}
	if len(got.IgnoreActions) != len(want.IgnoreActions) {
		t.Fatalf("IgnoreActions len: got %d, want %d", len(got.IgnoreActions), len(want.IgnoreActions))
	}
	for i := range got.IgnoreActions {
		if got.IgnoreActions[i] != want.IgnoreActions[i] {
			t.Errorf("IgnoreActions[%d]: got %p, want %p", i, got.IgnoreActions[i], want.IgnoreActions[i])
		}
	}
	if (got.GHES == nil) != (want.GHES == nil) {
		t.Errorf("GHES presence: got %v, want %v", got.GHES, want.GHES)
	}
	if got.GHES != nil && want.GHES != nil && got.GHES.APIURL != want.GHES.APIURL {
		t.Errorf("GHES.APIURL: got %q, want %q", got.GHES.APIURL, want.GHES.APIURL)
	}
	assertMinAgeEqual(t, got.MinAge, want.MinAge)
}

func assertMinAgeEqual(t *testing.T, got, want *MinAge) {
	t.Helper()
	if (got == nil) != (want == nil) {
		t.Fatalf("MinAge presence: got %v, want %v", got, want)
	}
	if got == nil {
		return
	}
	assertIntPtrEqual(t, "MinAge.Value", got.Value, want.Value)
	assertBoolPtrEqual(t, "MinAge.Always", got.Always, want.Always)
}

func assertIntPtrEqual(t *testing.T, label string, got, want *int) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Errorf("%s: got %v, want %v", label, got, want)
	case *got != *want:
		t.Errorf("%s: got %d, want %d", label, *got, *want)
	}
}

func assertBoolPtrEqual(t *testing.T, label string, got, want *bool) {
	t.Helper()
	switch {
	case got == nil && want == nil:
	case got == nil || want == nil:
		t.Errorf("%s: got %v, want %v", label, got, want)
	case *got != *want:
		t.Errorf("%s: got %v, want %v", label, *got, *want)
	}
}

func TestReader_ReadAndMerge(t *testing.T) { //nolint:funlen,gocognit,cyclop
	t.Parallel()

	t.Run("both files exist - project wins per field", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		globalContent := `version: 3
min_age:
  value: 7
  always: true
rules:
  - min_age: 0
    conditions:
      - expr: ActionRepoOwner == "suzuki-shunsuke"
`
		projectContent := `version: 3
separator: " # "
min_age:
  value: 3
`
		if err := afero.WriteFile(fs, "global.yaml", []byte(globalContent), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := afero.WriteFile(fs, ".pinact.yaml", []byte(projectContent), 0o644); err != nil {
			t.Fatal(err)
		}
		reader := NewReader(fs)
		cfg := &Config{}
		if err := reader.ReadAndMerge(cfg, ".pinact.yaml", "global.yaml"); err != nil {
			t.Fatalf("ReadAndMerge: %v", err)
		}
		if cfg.Separator != " # " {
			t.Errorf("Separator: got %q, want %q", cfg.Separator, " # ")
		}
		if cfg.MinAge == nil || cfg.MinAge.Value == nil || *cfg.MinAge.Value != 3 {
			t.Errorf("MinAge.Value: got %v, want 3", cfg.MinAge.Value)
		}
		if cfg.MinAge == nil || cfg.MinAge.Always == nil || !*cfg.MinAge.Always {
			t.Errorf("MinAge.Always: got %v, want true (carried from global)", cfg.MinAge.Always)
		}
		if len(cfg.Rules) != 1 {
			t.Fatalf("Rules len: got %d, want 1 (carried from global)", len(cfg.Rules))
		}
	})

	t.Run("global only", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		globalContent := `version: 3
min_age:
  always: true
`
		if err := afero.WriteFile(fs, "global.yaml", []byte(globalContent), 0o644); err != nil {
			t.Fatal(err)
		}
		reader := NewReader(fs)
		cfg := &Config{}
		if err := reader.ReadAndMerge(cfg, "", "global.yaml"); err != nil {
			t.Fatalf("ReadAndMerge: %v", err)
		}
		if cfg.MinAge == nil || cfg.MinAge.Always == nil || !*cfg.MinAge.Always {
			t.Errorf("MinAge.Always: got %v, want true", cfg.MinAge.Always)
		}
	})

	t.Run("global with abandoned version errors", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		globalContent := `version: 2
`
		projectContent := `version: 3
separator: " # "
`
		if err := afero.WriteFile(fs, "global.yaml", []byte(globalContent), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := afero.WriteFile(fs, ".pinact.yaml", []byte(projectContent), 0o644); err != nil {
			t.Fatal(err)
		}
		reader := NewReader(fs)
		cfg := &Config{}
		err := reader.ReadAndMerge(cfg, ".pinact.yaml", "global.yaml")
		if err == nil {
			t.Fatal("expected error for abandoned global version, got nil")
		}
		if msg := err.Error(); !contains(msg, "global.yaml") {
			t.Errorf("error should mention the source path: got %q", msg)
		}
	})

	t.Run("project with abandoned version errors", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		globalContent := `version: 3
`
		projectContent := `version: 2
`
		if err := afero.WriteFile(fs, "global.yaml", []byte(globalContent), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := afero.WriteFile(fs, ".pinact.yaml", []byte(projectContent), 0o644); err != nil {
			t.Fatal(err)
		}
		reader := NewReader(fs)
		cfg := &Config{}
		err := reader.ReadAndMerge(cfg, ".pinact.yaml", "global.yaml")
		if err == nil {
			t.Fatal("expected error for abandoned project version, got nil")
		}
		if msg := err.Error(); !contains(msg, ".pinact.yaml") {
			t.Errorf("error should mention the source path: got %q", msg)
		}
	})

	t.Run("neither file - cfg untouched", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		reader := NewReader(fs)
		cfg := &Config{Version: 99}
		if err := reader.ReadAndMerge(cfg, "", ""); err != nil {
			t.Fatalf("ReadAndMerge: %v", err)
		}
		if cfg.Version != 99 {
			t.Errorf("cfg should be untouched when both paths empty, got version %d", cfg.Version)
		}
	})
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
