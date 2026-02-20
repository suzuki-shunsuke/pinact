package di

import (
	"testing"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/gflag"
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
