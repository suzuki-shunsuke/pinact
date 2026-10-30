package run

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
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
	return readLockfile(afs, ".github/workflows/actions.lock", []string{".github/workflows/ci.yml"})
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
	if err := afero.WriteFile(afs, filepath.Join(root, ".github", "workflows", "actions.lock"), []byte(testLockfile), 0o644); err != nil {
		t.Fatal(err)
	}
	ci := filepath.Join(root, ".github", "workflows", "ci.yml")
	test := filepath.Join(root, ".github", "workflows", "test.yml")
	targets := readLockfiles(afs, []string{
		ci,
		test,
		filepath.Join(root, "action.yaml"),
		// No lockfile exists next to it.
		filepath.Join(other, ".github", "workflows", "ci.yml"),
	})
	if len(targets) != 2 {
		t.Fatalf("only workflows next to the lockfile should have targets: %v", targets)
	}
	if targets[ci] == nil || targets[ci].key != ".github/workflows/ci.yml" {
		t.Fatalf("the target of ci.yml is wrong: %v", targets[ci])
	}
	if targets[test] == nil || targets[test].key != ".github/workflows/test.yml" {
		t.Fatalf("the target of test.yml is wrong: %v", targets[test])
	}
	if targets[ci].state != targets[test].state {
		t.Fatal("the lockfile should be read once")
	}
	if targets[ci].state.err != nil {
		t.Fatal(targets[ci].state.err)
	}
}

func TestReadLockfile(t *testing.T) { //nolint:funlen
	t.Parallel()
	data := []struct {
		name    string
		content string // empty means the lockfile doesn't exist
		keys    []string
		isNil   bool
		isErr   bool
	}{
		{
			name:  "lockfile doesn't exist",
			keys:  []string{".github/workflows/ci.yml"},
			isNil: true,
		},
		{
			name:    "no workflow",
			content: "version: 'v0.0.3'\nfoo: bar\n",
			isNil:   true,
		},
		{
			name:    "valid",
			content: testLockfile,
			keys:    []string{".github/workflows/ci.yml"},
		},
		{
			name:    "invalid",
			content: "version: 'v0.0.3'\nfoo: bar\n",
			keys:    []string{".github/workflows/ci.yml"},
			isErr:   true,
		},
		{
			name:    "future schema version",
			content: "version: 'v99.0.0'\nworkflows: {}\ndependencies: {}\n",
			keys:    []string{".github/workflows/ci.yml"},
			isErr:   true,
		},
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
				if err := afero.WriteFile(afs, ".github/workflows/actions.lock", []byte(d.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			s := readLockfile(afs, ".github/workflows/actions.lock", d.keys)
			if d.isNil {
				if s != nil {
					t.Fatal("state should be nil")
				}
				return
			}
			if s == nil {
				t.Fatal("state should not be nil")
			}
			if (s.err != nil) != d.isErr {
				t.Fatalf("isErr is %v but the error is %v", d.isErr, s.err)
			}
		})
	}
}

func TestLockfileState_checkLockfile(t *testing.T) { //nolint:funlen
	t.Parallel()
	valid := newTestLockfileState(t, testLockfile)
	invalid := newTestLockfileState(t, "version: 'v0.0.3'\nfoo: bar\n")
	const sha = "3d3c42e5aac5ba805825da76410c181273ba90b1"
	data := []struct {
		name      string
		state     *lockfileState
		key       string
		action    *Action
		isHandled bool
		isErr     bool
	}{
		{
			name:   "lockfile doesn't exist",
			key:    ".github/workflows/ci.yml",
			action: &Action{Name: "actions/checkout", Version: "v6"},
		},
		{
			name:   "not a workflow file",
			state:  valid,
			action: &Action{Name: "actions/checkout", Version: "v6"},
		},
		{
			name:   "reusable workflow",
			state:  valid,
			key:    ".github/workflows/ci.yml",
			action: &Action{Name: "suzuki-shunsuke/go-release-workflow/.github/workflows/release.yaml", Version: "v10.2.0"},
		},
		{
			name:   "workflow isn't onboarded",
			state:  valid,
			key:    ".github/workflows/test.yml",
			action: &Action{Name: "actions/setup-go", Version: "v6"},
		},
		{
			name:      "locked",
			state:     valid,
			key:       ".github/workflows/ci.yml",
			action:    &Action{Name: "actions/checkout", Version: "v6"},
			isHandled: true,
		},
		{
			name:      "sub-action path and casing",
			state:     valid,
			key:       ".github/workflows/ci.yml",
			action:    &Action{Name: "Actions/Cache/save", Version: "v4.2.0"},
			isHandled: true,
		},
		{
			name:      "not locked in an onboarded workflow",
			state:     valid,
			key:       ".github/workflows/ci.yml",
			action:    &Action{Name: "actions/setup-go", Version: "v6"},
			isHandled: true,
			isErr:     true,
		},
		{
			name:      "pinned by SHA but not locked in an onboarded workflow",
			state:     valid,
			key:       ".github/workflows/ci.yml",
			action:    &Action{Name: "actions/checkout", Version: sha},
			isHandled: true,
			isErr:     true,
		},
		{
			name:      "unreadable lockfile and pinned by SHA",
			state:     invalid,
			key:       ".github/workflows/ci.yml",
			action:    &Action{Name: "actions/checkout", Version: sha},
			isHandled: true,
		},
		{
			name:      "unreadable lockfile and not pinned",
			state:     invalid,
			key:       ".github/workflows/ci.yml",
			action:    &Action{Name: "actions/checkout", Version: "v6"},
			isHandled: true,
			isErr:     true,
		},
		{
			name:   "unreadable lockfile and reusable workflow",
			state:  invalid,
			key:    ".github/workflows/ci.yml",
			action: &Action{Name: "suzuki-shunsuke/go-release-workflow/.github/workflows/release.yaml", Version: "v10.2.0"},
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			handled, err := d.state.checkLockfile(d.key, d.action)
			if handled != d.isHandled {
				t.Fatalf("handled is %v, wanted %v", handled, d.isHandled)
			}
			if (err != nil) != d.isErr {
				t.Fatalf("isErr is %v but the error is %v", d.isErr, err)
			}
			if err != nil && !errors.Is(err, ErrUnfixable) {
				t.Fatalf("the error should be ErrUnfixable: %v", err)
			}
		})
	}
}

// TestController_processLines_lockfile tests that locked actions aren't changed
// and unlocked actions in an onboarded workflow make pinact exit with ExitCodeUnfixable.
// The GitHub API isn't called because no tag is registered in the fake service.
func TestController_processLines_lockfile(t *testing.T) {
	t.Parallel()
	data := []struct {
		name         string
		line         string
		wantExitCode int
	}{
		{
			name:         "locked",
			line:         "      - uses: actions/checkout@v6",
			wantExitCode: ExitCodeOK,
		},
		{
			name:         "not locked",
			line:         "      - uses: actions/setup-go@v6",
			wantExitCode: ExitCodeUnfixable,
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			ctrl := New(&github.RepositoriesServiceImpl{}, nil, afero.NewMemMapFs(), &config.Config{}, &ParamRun{Stderr: &bytes.Buffer{}, Fix: true})
			ctrl.lockfiles = map[string]*lockfileTarget{
				".github/workflows/ci.yml": {state: newTestLockfileState(t, testLockfile), key: ".github/workflows/ci.yml"},
			}
			lines := []string{d.line}
			changed, exitCode := ctrl.processLines(t.Context(), slog.New(slog.DiscardHandler), ".github/workflows/ci.yml", lines)
			if changed {
				t.Fatalf("the line shouldn't be changed: %s", lines[0])
			}
			if exitCode != d.wantExitCode {
				t.Fatalf("exit code is %d, wanted %d", exitCode, d.wantExitCode)
			}
		})
	}
}
