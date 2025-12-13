package config

import (
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
		version int
		wantErr bool
	}{
		{name: "valid pattern", file: &File{Pattern: "*.yaml"}, version: 3, wantErr: false},
		{name: "empty pattern", file: &File{Pattern: ""}, version: 3, wantErr: true},
		{name: "invalid version", file: &File{Pattern: "*.yaml"}, version: 0, wantErr: true},
		{name: "invalid glob pattern", file: &File{Pattern: "[invalid"}, version: 3, wantErr: true},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			err := d.file.Init(d.version)
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
		version int
		wantErr bool
	}{
		{name: "valid", ia: &IgnoreAction{Name: "actions/checkout", Ref: "v4"}, version: 3, wantErr: false},
		{name: "empty name", ia: &IgnoreAction{Name: "", Ref: "v4"}, version: 3, wantErr: true},
		{name: "empty ref", ia: &IgnoreAction{Name: "actions/checkout", Ref: ""}, version: 3, wantErr: true},
		{name: "invalid name regex", ia: &IgnoreAction{Name: "[invalid", Ref: "v4"}, version: 3, wantErr: true},
		{name: "invalid ref regex", ia: &IgnoreAction{Name: "actions/checkout", Ref: "[invalid"}, version: 3, wantErr: true},
		{name: "invalid version", ia: &IgnoreAction{Name: "actions/checkout", Ref: "v4"}, version: 0, wantErr: true},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			err := d.ia.Init(d.version)
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

func TestReader_Read(t *testing.T) {
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
