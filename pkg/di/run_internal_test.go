package di

import (
	"testing"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/gflag"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
)

func Test_compileRegexps(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		got, err := compileRegexps([]string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("wanted 0, got %d", len(got))
		}
	})

	t.Run("valid regexes", func(t *testing.T) {
		t.Parallel()
		got, err := compileRegexps([]string{"^foo", "bar$"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("wanted 2, got %d", len(got))
		}
		if !got[0].MatchString("foobar") {
			t.Error("expected ^foo to match 'foobar'")
		}
		if !got[1].MatchString("foobar") {
			t.Error("expected bar$ to match 'foobar'")
		}
	})

	t.Run("invalid regex", func(t *testing.T) {
		t.Parallel()
		if _, err := compileRegexps([]string{"[invalid"}); err == nil {
			t.Error("expected error, got nil")
		}
	})
}

func Test_buildParam_default(t *testing.T) {
	t.Parallel()
	flags := &Flags{GlobalFlags: &gflag.GlobalFlags{}, Args: []string{"test.yaml"}, CWD: "/tmp"}
	got, err := buildParam(flags, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Fix {
		t.Error("Fix: wanted true, got false")
	}
	if len(got.WorkflowFilePaths) != 1 {
		t.Errorf("WorkflowFilePaths: wanted 1, got %d", len(got.WorkflowFilePaths))
	}
}

func Test_buildParam_checkMode(t *testing.T) {
	t.Parallel()
	flags := &Flags{GlobalFlags: &gflag.GlobalFlags{}, Check: true}
	got, err := buildParam(flags, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Fix {
		t.Error("Fix: wanted false, got true")
	}
	if !got.Check {
		t.Error("Check: wanted true, got false")
	}
}

func Test_buildParam_diffMode(t *testing.T) {
	t.Parallel()
	flags := &Flags{GlobalFlags: &gflag.GlobalFlags{}, Diff: true}
	got, err := buildParam(flags, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Fix {
		t.Error("Fix: wanted false, got true")
	}
}

func Test_buildParam_explicitFix(t *testing.T) {
	t.Parallel()
	flags := &Flags{GlobalFlags: &gflag.GlobalFlags{}, Check: true, Fix: true, FixCount: 1}
	got, err := buildParam(flags, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Fix {
		t.Error("Fix: wanted true, got false")
	}
}

func Test_buildParam_invalidRegex(t *testing.T) {
	t.Parallel()
	t.Run("invalid include", func(t *testing.T) {
		t.Parallel()
		flags := &Flags{GlobalFlags: &gflag.GlobalFlags{}, Include: []string{"[invalid"}}
		if _, err := buildParam(flags, nil); err == nil {
			t.Error("expected error, got nil")
		}
	})
	t.Run("invalid exclude", func(t *testing.T) {
		t.Parallel()
		flags := &Flags{GlobalFlags: &gflag.GlobalFlags{}, Exclude: []string{"[invalid"}}
		if _, err := buildParam(flags, nil); err == nil {
			t.Error("expected error, got nil")
		}
	})
}

//nolint:funlen
func Test_getMinAge(t *testing.T) {
	t.Parallel()

	t.Run("Invalid env var", func(t *testing.T) {
		_, err := getMinAge(&config.Config{}, &Flags{}, func(string) string {
			return "not an integer"
		})

		if err == nil {
			t.Error("expected error, got nil")
		}
	})

	data := []struct {
		name        string
		cfg         *config.Config
		flags       *Flags
		env         string
		expectedAge int
	}{
		{
			// Config has 'min_age: 14' but user passes '--min-age 7'
			// => cooldown must be 7 days
			name:        "flag takes precedence",
			cfg:         &config.Config{Updates: &config.Updates{MinAge: 14}},
			flags:       &Flags{MinAge: 7},
			env:         "",
			expectedAge: 7,
		},
		{
			name:        "config is used when --min-age flag isn't provided",
			cfg:         &config.Config{Updates: &config.Updates{MinAge: 14}},
			flags:       &Flags{},
			env:         "",
			expectedAge: 14,
		},
		{
			name:        "env value takes precedence over config",
			cfg:         &config.Config{Updates: &config.Updates{MinAge: 14}},
			flags:       &Flags{},
			env:         "3",
			expectedAge: 3,
		},
		{
			// When env var + config + flag are provided, then the flag takes precedence
			name:        "flag takes precedence over env var & over the config",
			cfg:         &config.Config{Updates: &config.Updates{MinAge: 14}},
			flags:       &Flags{MinAge: 3},
			env:         "5",
			expectedAge: 3,
		},
		{
			// When MinAge isn't set (flags + config + env var), then it should use
			// the default value
			name:        "uses the default value when none provided",
			cfg:         &config.Config{},
			flags:       &Flags{},
			env:         "",
			expectedAge: config.DefaultMinAge,
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			got, err := getMinAge(d.cfg, d.flags, func(string) string {
				return d.env
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != d.expectedAge {
				t.Errorf("wanted %d, got %d", d.expectedAge, got)
			}
		})
	}
}
