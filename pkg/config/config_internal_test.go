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
		if cfg.MinAge.Value != 60 {
			t.Errorf("MinAge.Value: wanted 60, got %d", cfg.MinAge.Value)
		}
		if !cfg.MinAge.Always {
			t.Errorf("MinAge.Always: wanted true, got false")
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

func Test_getConfigPath(t *testing.T) {
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
			got, err := getConfigPath(fs)
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
		name       string
		rules      []*Rule
		wantIgnore bool
		wantMinAge *int
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
		})
	}
}
