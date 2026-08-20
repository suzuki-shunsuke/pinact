package updaterepos //nolint:testpackage // tests unexported helpers

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/github"
)

func TestControllerSelected(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		param Param
		repo  *github.Repository
		want  bool
	}{
		"active repository": {
			repo: &github.Repository{FullName: github.Ptr("acme/service")},
			want: true,
		},
		"archived repository": {
			repo: &github.Repository{FullName: github.Ptr("acme/service"), Archived: github.Ptr(true)},
		},
		"included archived repository": {
			param: Param{IncludeArchived: true},
			repo:  &github.Repository{FullName: github.Ptr("acme/service"), Archived: github.Ptr(true)},
			want:  true,
		},
		"fork repository": {
			repo: &github.Repository{FullName: github.Ptr("acme/service"), Fork: github.Ptr(true)},
		},
		"excluded repository wins": {
			param: Param{IncludeRepositories: []*regexp.Regexp{regexp.MustCompile("acme/.*")}, ExcludeRepositories: []*regexp.Regexp{regexp.MustCompile("service")}},
			repo:  &github.Repository{FullName: github.Ptr("acme/service")},
		},
		"included repository": {
			param: Param{IncludeRepositories: []*regexp.Regexp{regexp.MustCompile("acme/service")}},
			repo:  &github.Repository{FullName: github.Ptr("acme/service")},
			want:  true,
		},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			controller := New(nil, test.param, nil, nil)
			if got := controller.selected(test.repo); got != test.want {
				t.Errorf("selected() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNewDefaults(t *testing.T) {
	t.Parallel()
	controller := New(nil, Param{}, nil, nil)
	if diff := cmp.Diff(defaultBranch, controller.param.Branch); diff != "" {
		t.Errorf("branch (-want +got):\n%s", diff)
	}
	if controller.param.Concurrency != DefaultConcurrency {
		t.Errorf("concurrency = %d, want %d", controller.param.Concurrency, DefaultConcurrency)
	}
}

func TestWorkflowFilesUsesConfiguration(t *testing.T) {
	t.Parallel()
	workdir := t.TempDir()
	configPath := filepath.Join(workdir, ".pinact.yaml")
	if err := os.WriteFile(filepath.Join(workdir, "README.md"), []byte("uses: actions/checkout@v4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := workflowFiles(workdir, &config.Config{Files: []*config.File{{Pattern: "README.md"}}}, configPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(workdir, "README.md")}
	if diff := cmp.Diff(want, files); diff != "" {
		t.Errorf("workflowFiles() (-want +got):\n%s", diff)
	}
}

func TestReadConfigMergesGlobalConfig(t *testing.T) {
	workdir := t.TempDir()
	globalPath := filepath.Join(t.TempDir(), "pinact.yaml")
	if err := os.WriteFile(globalPath, []byte("version: 3\nghes:\n  api_url: https://ghes.example.com/api/v3\n  fallback: true\nignore_actions:\n  - name: actions/checkout\n    ref: .*\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workdir, ".pinact.yaml"), []byte("version: 3\nignore_actions:\n  - name: actions/setup-go\n    ref: .*\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PINACT_GLOBAL_CONFIG", globalPath)
	cfg, configPath, err := readConfig(workdir)
	if err != nil {
		t.Fatal(err)
	}
	wantPath := filepath.Join(workdir, ".pinact.yaml")
	ghesURL := ""
	ghesFallback := false
	if cfg.GHES != nil {
		ghesURL = cfg.GHES.APIURL
		ghesFallback = cfg.GHES.Fallback
	}
	names := make([]string, 0, len(cfg.IgnoreActions))
	for _, action := range cfg.IgnoreActions {
		names = append(names, action.Name)
	}
	if configPath != wantPath || ghesURL != "https://ghes.example.com/api/v3" || !ghesFallback {
		t.Errorf("configPath=%s GHES=%+v, want path %s with global GHES fallback", configPath, cfg.GHES, wantPath)
	}
	if diff := cmp.Diff([]string{"actions/checkout", "actions/setup-go"}, names); diff != "" {
		t.Errorf("IgnoreActions (-want +got):\n%s", diff)
	}
}

func TestWorkflowFilesUsesWorkdirWhenConfigPathEmpty(t *testing.T) {
	t.Parallel()
	workdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workdir, "README.md"), []byte("uses: actions/checkout@v4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := workflowFiles(workdir, &config.Config{Files: []*config.File{{Pattern: "README.md"}}}, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(workdir, "README.md")}
	if diff := cmp.Diff(want, files); diff != "" {
		t.Errorf("workflowFiles() (-want +got):\n%s", diff)
	}
}
