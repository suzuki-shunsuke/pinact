package run

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/github"
)

const testLockfile = `version: 'v0.0.3'
workflows:
  '.github/workflows/ci.yml':
    - 'actions/cache@v4.2.0'
    - 'actions/checkout@v6'
dependencies:
  'actions/cache@v4.2.0':
    ref: 'v4.2.0'
    commit: 'sha1-1bd1e32a3bdc45362d1e726936510720a7c30a57'
    owner_id: 44036562
    repo_id: 207226787
  'actions/checkout@v6':
    ref: 'v6'
    commit: 'sha1-3d3c42e5aac5ba805825da76410c181273ba90b1'
    owner_id: 44036562
    repo_id: 197814629
`

func newTestLockfileState(t *testing.T, content string) *lockfileState {
	t.Helper()
	afs := afero.NewMemMapFs()
	if err := afero.WriteFile(afs, ".github/workflows/actions.lock", []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := readLockfile(afs, ".github/workflows/actions.lock", []string{".github/workflows/ci.yml"})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// writeFiles writes files whose paths are relative to root and slash-separated.
func writeFiles(t *testing.T, afs afero.Fs, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		if err := afero.WriteFile(afs, filepath.Join(root, filepath.FromSlash(name)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWorkflowDirAndKey(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	data := []struct {
		filePath string
		dir      string
		key      string
	}{
		{filePath: ".github/workflows/ci.yml", dir: filepath.Join(cwd, ".github", "workflows"), key: ".github/workflows/ci.yml"},
		{filePath: "./.github/workflows/ci.yaml", dir: filepath.Join(cwd, ".github", "workflows"), key: ".github/workflows/ci.yaml"},
		{filePath: filepath.Join(root, ".github", "workflows", "ci.yml"), dir: filepath.Join(root, ".github", "workflows"), key: ".github/workflows/ci.yml"},
		// The repository root is derived from the file path, not from the working directory.
		{filePath: filepath.Join("foo", ".github", "workflows", "ci.yml"), dir: filepath.Join(cwd, "foo", ".github", "workflows"), key: ".github/workflows/ci.yml"},
		{filePath: "action.yaml"},
		{filePath: ".github/workflows/sub/ci.yml"},
		{filePath: ".github/workflows/actions.lock"},
		{filePath: ".github/ci.yml"},
	}
	for _, d := range data {
		t.Run(d.filePath, func(t *testing.T) {
			t.Parallel()
			dir, key := workflowDirAndKey(d.filePath)
			if dir != d.dir || key != d.key {
				t.Fatalf("got (%q, %q), wanted (%q, %q)", dir, key, d.dir, d.key)
			}
		})
	}
}

func TestReadLockfiles(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "repo")
	other := filepath.Join(t.TempDir(), "other")
	afs := afero.NewMemMapFs()
	writeFiles(t, afs, root, map[string]string{".github/workflows/actions.lock": testLockfile})
	ci := filepath.Join(root, ".github", "workflows", "ci.yml")
	buf := &bytes.Buffer{}
	targets, err := readLockfiles(afs, slog.New(slog.NewTextHandler(buf, nil)), []string{
		ci,
		// It isn't onboarded, so it's pinned as usual with a warning.
		filepath.Join(root, ".github", "workflows", "test.yml"),
		// It isn't used as a self repository action.
		filepath.Join(root, "action.yaml"),
		// No lockfile exists next to it.
		filepath.Join(other, ".github", "workflows", "ci.yml"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("only the onboarded workflow should be a target: %v", targets)
	}
	if targets[ci] == nil || !slices.Equal(targets[ci].keys, []string{".github/workflows/ci.yml"}) {
		t.Fatalf("the target of ci.yml is wrong: %v", targets[ci])
	}
	if out := buf.String(); !strings.Contains(out, "the workflow isn't in the lockfile") || !strings.Contains(out, ".github/workflows/test.yml") {
		t.Fatalf("a warning for test.yml should be output: %s", out)
	}
}

func TestReadLockfiles_invalid(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "repo")
	afs := afero.NewMemMapFs()
	writeFiles(t, afs, root, map[string]string{".github/workflows/actions.lock": "version: 'v0.0.3'\nfoo: bar\n"})
	if _, err := readLockfiles(afs, slog.New(slog.DiscardHandler), []string{filepath.Join(root, ".github", "workflows", "ci.yml")}); err == nil {
		t.Fatal("an error should be returned if the lockfile can't be read")
	}
	// The lockfile isn't read if no target file concerns it.
	if _, err := readLockfiles(afs, slog.New(slog.DiscardHandler), []string{filepath.Join(root, "action.yaml")}); err != nil {
		t.Fatal(err)
	}
}

func TestReadLockfile(t *testing.T) {
	t.Parallel()
	data := []struct {
		name    string
		content string // empty means the lockfile doesn't exist
		keys    []string
		isNil   bool
		isErr   bool
	}{
		{name: "lockfile doesn't exist", keys: []string{".github/workflows/ci.yml"}, isNil: true},
		{name: "no workflow", content: "version: 'v0.0.3'\nfoo: bar\n", isNil: true},
		{name: "valid", content: testLockfile, keys: []string{".github/workflows/ci.yml"}},
		{name: "invalid", content: "version: 'v0.0.3'\nfoo: bar\n", keys: []string{".github/workflows/ci.yml"}, isErr: true},
		{name: "future schema version", content: "version: 'v99.0.0'\nworkflows: {}\ndependencies: {}\n", keys: []string{".github/workflows/ci.yml"}, isErr: true},
		{
			// actions/setup-go@v6 lacks commit, but ci.yml doesn't use it.
			name: "broken entry that no workflow uses",
			content: testLockfile + `  'actions/setup-go@v6':
    ref: 'v6'
    owner_id: 44036562
    repo_id: 192624594
`,
			keys: []string{".github/workflows/ci.yml"},
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			afs := afero.NewMemMapFs()
			if d.content != "" {
				writeFiles(t, afs, "", map[string]string{".github/workflows/actions.lock": d.content})
			}
			s, err := readLockfile(afs, ".github/workflows/actions.lock", d.keys)
			if (err != nil) != d.isErr {
				t.Fatalf("isErr is %v but the error is %v", d.isErr, err)
			}
			if err != nil {
				return
			}
			if (s == nil) != d.isNil {
				t.Fatalf("isNil is %v but the state is %v", d.isNil, s)
			}
		})
	}
}

func TestReadLockfiles_selfRepository(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "repo")
	afs := afero.NewMemMapFs()
	writeFiles(t, afs, root, map[string]string{
		".github/workflows/actions.lock": testLockfile,
		".github/workflows/ci.yml": `jobs:
  call:
    uses: $/.github/workflows/reusable.yml
  build:
    steps:
      - uses: $/.github/actions/setup
`,
		// It isn't onboarded but uses the same action file.
		".github/workflows/release.yml":      "jobs:\n  build:\n    steps:\n      - uses: $/.github/actions/setup\n",
		".github/workflows/reusable.yml":     "on: workflow_call\n",
		".github/actions/setup/action.yaml":  "runs:\n  steps:\n    - uses: actions/cache@v4.2.0\n    - uses: \"$/.github/actions/nested\"\n",
		".github/actions/nested/action.yml":  "runs:\n  steps:\n    - uses: actions/checkout@v6\n",
		".github/actions/unused/action.yaml": "runs:\n  steps:\n    - uses: actions/checkout@v6\n",
		"action.yaml":                        "runs:\n  steps:\n    - uses: actions/checkout@v6\n",
	})
	setup := filepath.Join(root, ".github", "actions", "setup", "action.yaml")
	nested := filepath.Join(root, ".github", "actions", "nested", "action.yml")
	buf := &bytes.Buffer{}
	targets, err := readLockfiles(afs, slog.New(slog.NewTextHandler(buf, nil)), []string{
		setup,
		nested,
		filepath.Join(root, ".github", "actions", "unused", "action.yaml"),
		filepath.Join(root, "action.yaml"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 2 {
		t.Fatalf("only action files used as self repository actions by onboarded workflows should be targets: %v", targets)
	}
	for _, f := range []string{setup, nested} {
		if targets[f] == nil || !slices.Equal(targets[f].keys, []string{".github/workflows/ci.yml"}) {
			t.Fatalf("the target of %s is wrong: %v", f, targets[f])
		}
	}
	if out := buf.String(); strings.Count(out, "the workflow isn't in the lockfile") != 1 || !strings.Contains(out, ".github/workflows/release.yml") {
		t.Fatalf("a warning for release.yml should be output once: %s", out)
	}
}

func TestLockfileState_checkLockfile(t *testing.T) { //nolint:funlen
	t.Parallel()
	s := newTestLockfileState(t, testLockfile+`  'actions/setup-go@v6':
    ref: 'v6'
    commit: 'sha1-924ae3a1cded613372ab5595356fb5720e22ba16'
    owner_id: 44036562
    repo_id: 192624594
`)
	s.file.Workflows[".github/workflows/go.yml"] = []string{"actions/setup-go@v6"}
	const sha = "3d3c42e5aac5ba805825da76410c181273ba90b1"
	data := []struct {
		name      string
		state     *lockfileState
		keys      []string
		action    *Action
		isHandled bool
		warning   string // a substring of the warning. Empty means no warning
	}{
		{
			name:   "lockfile doesn't exist",
			keys:   []string{".github/workflows/ci.yml"},
			action: &Action{Name: "actions/checkout", Version: "v6"},
		},
		{
			name:   "no onboarded workflow",
			state:  s,
			action: &Action{Name: "actions/checkout", Version: "v6"},
		},
		{
			name:   "reusable workflow",
			state:  s,
			keys:   []string{".github/workflows/ci.yml"},
			action: &Action{Name: "suzuki-shunsuke/go-release-workflow/.github/workflows/release.yaml", Version: "v10.2.0"},
		},
		{
			name:      "recorded",
			state:     s,
			keys:      []string{".github/workflows/ci.yml"},
			action:    &Action{Name: "actions/checkout", Version: "v6"},
			isHandled: true,
		},
		{
			name:      "sub-action path and casing",
			state:     s,
			keys:      []string{".github/workflows/ci.yml"},
			action:    &Action{Name: "Actions/Cache/save", Version: "v4.2.0"},
			isHandled: true,
		},
		{
			name:      "not recorded",
			state:     s,
			keys:      []string{".github/workflows/ci.yml"},
			action:    &Action{Name: "actions/setup-go", Version: "v6"},
			isHandled: true,
			warning:   "the lockfile doesn't record actions/setup-go@v6 for .github/workflows/ci.yml",
		},
		{
			name:      "pinned by SHA but not recorded",
			state:     s,
			keys:      []string{".github/workflows/ci.yml"},
			action:    &Action{Name: "actions/checkout", Version: sha},
			isHandled: true,
			warning:   "actions/checkout@" + sha,
		},
		{
			name:      "not recorded for one of the workflows",
			state:     s,
			keys:      []string{".github/workflows/ci.yml", ".github/workflows/go.yml"},
			action:    &Action{Name: "actions/checkout", Version: "v6"},
			isHandled: true,
			warning:   "for .github/workflows/go.yml,",
		},
		{
			name:      "not recorded for all of the workflows",
			state:     s,
			keys:      []string{".github/workflows/ci.yml", ".github/workflows/go.yml"},
			action:    &Action{Name: "actions/setup-node", Version: "v6"},
			isHandled: true,
			warning:   "for .github/workflows/ci.yml, .github/workflows/go.yml,",
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			handled, warning := d.state.checkLockfile(d.keys, d.action)
			if handled != d.isHandled {
				t.Fatalf("handled is %v, wanted %v", handled, d.isHandled)
			}
			if d.warning == "" {
				if warning != "" {
					t.Fatalf("no warning is expected: %s", warning)
				}
				return
			}
			if !strings.Contains(warning, d.warning) {
				t.Fatalf("the warning %q should contain %q", warning, d.warning)
			}
		})
	}
}

// TestController_processLines_lockfile tests that actions in an onboarded workflow aren't changed
// whether the lockfile records them or not, and pinact doesn't fail.
// The GitHub API isn't called because no tag is registered in the fake service.
func TestController_processLines_lockfile(t *testing.T) {
	t.Parallel()
	for _, line := range []string{
		"      - uses: actions/checkout@v6",
		"      - uses: actions/setup-go@v6",
	} {
		t.Run(line, func(t *testing.T) {
			t.Parallel()
			ctrl := New(&github.RepositoriesServiceImpl{}, nil, afero.NewMemMapFs(), &config.Config{}, &ParamRun{Stderr: &bytes.Buffer{}, Fix: true})
			ctrl.lockfiles = map[string]*lockfileTarget{
				".github/workflows/ci.yml": {state: newTestLockfileState(t, testLockfile), keys: []string{".github/workflows/ci.yml"}},
			}
			lines := []string{line}
			changed, exitCode := ctrl.processLines(t.Context(), slog.New(slog.DiscardHandler), ".github/workflows/ci.yml", lines)
			if changed {
				t.Fatalf("the line shouldn't be changed: %s", lines[0])
			}
			if exitCode != ExitCodeOK {
				t.Fatalf("exit code is %d, wanted %d", exitCode, ExitCodeOK)
			}
		})
	}
}
