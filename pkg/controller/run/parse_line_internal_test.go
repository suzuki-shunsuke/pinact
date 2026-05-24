package run

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/github"
)

func Test_parseAction(t *testing.T) { //nolint:funlen
	t.Parallel()
	data := []struct {
		name string
		line string
		exp  *Action
	}{
		{
			name: "unrelated",
			line: "unrelated",
		},
		{
			name: "checkout v3",
			line: "  - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3",
			exp: &Action{
				Uses:                    "  - uses: ",
				Name:                    "actions/checkout",
				Version:                 "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionCommentSeparator: " # ",
				VersionComment:          "v3",
			},
		},
		{
			name: "checkout v2",
			line: "  uses: actions/checkout@v2",
			exp: &Action{
				Uses:    "  uses: ",
				Name:    "actions/checkout",
				Version: "v2",
			},
		},
		{
			name: "checkout v3 (single quote)",
			line: `  - "uses": 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # v3`,
			exp: &Action{
				Uses:                    `  - "uses": `,
				Name:                    "actions/checkout",
				Version:                 "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionCommentSeparator: " # ",
				VersionComment:          "v3",
				Quote:                   "'",
			},
		},
		{
			name: "checkout v3 (double quote)",
			line: `  - 'uses': "actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab" # v3`,
			exp: &Action{
				Uses:                    `  - 'uses': `,
				Name:                    "actions/checkout",
				Version:                 "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionCommentSeparator: " # ",
				VersionComment:          "v3",
				Quote:                   `"`,
			},
		},
		{
			name: "checkout v2 (single quote)",
			line: `  "uses": 'actions/checkout@v2'`,
			exp: &Action{
				Uses:           `  "uses": `,
				Name:           "actions/checkout",
				Version:        "v2",
				VersionComment: "",
				Quote:          `'`,
			},
		},
		{
			name: "checkout v2 (double quote)",
			line: `  'uses': "actions/checkout@v2"`,
			exp: &Action{
				Uses:           `  'uses': `,
				Name:           "actions/checkout",
				Version:        "v2",
				VersionComment: "",
				Quote:          `"`,
			},
		},
		{
			name: "tag=",
			line: `      - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # tag=v3`,
			exp: &Action{
				Uses:                    `      - uses: `,
				Name:                    "actions/checkout",
				Version:                 "83b7061638ee4956cf7545a6f7efe594e5ad0247",
				VersionCommentSeparator: " # tag=",
				VersionComment:          "v3",
				Quote:                   "",
			},
		},
		{
			name: "multi-space after dash",
			line: "    -   uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3",
			exp: &Action{
				Uses:                    "    -   uses: ",
				Name:                    "actions/checkout",
				Version:                 "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionCommentSeparator: " # ",
				VersionComment:          "v3",
			},
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			act := parseAction(d.line)
			if diff := cmp.Diff(d.exp, act); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestController_parseLine(t *testing.T) { //nolint:funlen
	t.Parallel()
	data := []struct {
		name  string
		line  string
		exp   string
		isErr bool
	}{
		{
			name: "unrelated",
			line: "unrelated",
		},
		{
			name: "checkout v3",
			line: "  - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3",
			exp:  "  - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3.5.2",
		},
		{
			name: "checkout v2",
			line: "  uses: actions/checkout@v2",
			exp:  "  uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.7.0",
		},
		{
			name: "single quote",
			line: `  - "uses": 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # v3`,
			exp:  `  - "uses": 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # v3.5.2`,
		},
		{
			name: "double quote",
			line: `  - 'uses': "actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab" # v3`,
			exp:  `  - 'uses': "actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab" # v3.5.2`,
		},
		{
			name: "checkout v2 (single quote)",
			line: `  "uses": 'actions/checkout@v2'`,
			exp:  `  "uses": 'actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5' # v2.7.0`,
		},
		{
			name: "pinned SHA without comment",
			line: "  - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
			exp:  "  - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3.5.2",
		},
		{
			name: "pinned SHA without comment (quoted)",
			line: `  - 'uses': "actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab"`,
			exp:  `  - 'uses': "actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab" # v3.5.2`,
		},
		{
			name:  "pinned SHA without comment - no matching tag",
			line:  "  - uses: actions/checkout@aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			isErr: true,
		},
		{
			name: "multi-space after dash",
			line: "      -   uses: actions/checkout@v3",
			exp:  "      -   uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3.5.2",
		},
	}
	logger := slog.New(slog.DiscardHandler)
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			ctrl := New(&github.RepositoriesServiceImpl{
				Tags: map[string]*github.ListTagsResult{
					"actions/checkout/0": {
						Tags: []*github.RepositoryTag{
							{
								Name: new("v3"),
								Commit: &github.Commit{
									SHA: new("8e5e7e5ab8b370d6c329ec480221332ada57f0ab"),
								},
							},
							{
								Name: new("v3.5.2"),
								Commit: &github.Commit{
									SHA: new("8e5e7e5ab8b370d6c329ec480221332ada57f0ab"),
								},
							},
							{
								Name: new("v2"),
								Commit: &github.Commit{
									SHA: new("ee0669bd1cc54295c223e0bb666b733df41de1c5"),
								},
							},
							{
								Name: new("v2.7.0"),
								Commit: &github.Commit{
									SHA: new("ee0669bd1cc54295c223e0bb666b733df41de1c5"),
								},
							},
						},
						Response: &github.Response{},
					},
				},
				Releases: map[string]*github.ListReleasesResult{
					"actions/checkout/0": {
						Releases: []*github.RepositoryRelease{}, // Empty releases forces fallback to tags
						Response: &github.Response{},
					},
				},
				Commits: map[string]*github.GetCommitSHA1Result{
					"actions/checkout/v3": {
						SHA: "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
					},
					"actions/checkout/v2": {
						SHA: "ee0669bd1cc54295c223e0bb666b733df41de1c5",
					},
				},
			}, nil, fs, &config.Config{
				Separator: " # ",
			}, &ParamRun{})
			line, err := ctrl.parseLine(t.Context(), logger, d.line)
			if err != nil {
				if d.isErr {
					return
				}
				t.Fatal(err)
			}
			if line != d.exp {
				t.Fatalf(`wanted %s, got %s`, d.exp, line)
			}
		})
	}
}

func TestController_parseLine_addMissingComment(t *testing.T) { //nolint:funlen
	t.Parallel()
	sha := "8e5e7e5ab8b370d6c329ec480221332ada57f0ab"
	data := []struct {
		name    string
		tags    []*github.RepositoryTag
		commits map[string]*github.GetCommitSHA1Result
		line    string
		exp     string
		wantErr error
	}{
		{
			name: "semver tag found directly",
			tags: []*github.RepositoryTag{
				{
					Name:   new("v3.5.2"),
					Commit: &github.Commit{SHA: new(sha)},
				},
			},
			line: "  - uses: actions/checkout@" + sha,
			exp:  "  - uses: actions/checkout@" + sha + " # v3.5.2",
		},
		{
			name: "only short semver tag exists",
			tags: []*github.RepositoryTag{
				{
					Name:   new("v3"),
					Commit: &github.Commit{SHA: new(sha)},
				},
			},
			line: "  - uses: actions/checkout@" + sha,
			exp:  "  - uses: actions/checkout@" + sha + " # v3",
		},
		{
			name: "non-semver tag before semver tag - pinned SHA",
			tags: []*github.RepositoryTag{
				{
					Name:   new("latest"),
					Commit: &github.Commit{SHA: new(sha)},
				},
				{
					Name:   new("v2"),
					Commit: &github.Commit{SHA: new(sha)},
				},
				{
					Name:   new("v2.11.5"),
					Commit: &github.Commit{SHA: new(sha)},
				},
			},
			line: "  - uses: actions/checkout@" + sha,
			exp:  "  - uses: actions/checkout@" + sha + " # v2.11.5",
		},
		{
			name: "non-semver tag before semver tag - short version",
			tags: []*github.RepositoryTag{
				{
					Name:   new("latest"),
					Commit: &github.Commit{SHA: new(sha)},
				},
				{
					Name:   new("v2"),
					Commit: &github.Commit{SHA: new(sha)},
				},
				{
					Name:   new("v2.11.5"),
					Commit: &github.Commit{SHA: new(sha)},
				},
			},
			commits: map[string]*github.GetCommitSHA1Result{
				"actions/checkout/v2": {SHA: sha},
			},
			line: "  - uses: actions/checkout@v2",
			exp:  "  - uses: actions/checkout@" + sha + " # v2.11.5",
		},
		{
			name: "issue 1447 - svenstaro/upload-release-action picks latest instead of semver",
			tags: []*github.RepositoryTag{
				{
					Name:   new("latest"),
					Commit: &github.Commit{SHA: new(sha)},
				},
				{
					Name:   new("v2"),
					Commit: &github.Commit{SHA: new(sha)},
				},
				{
					Name:   new("2.11.5"),
					Commit: &github.Commit{SHA: new(sha)},
				},
			},
			commits: map[string]*github.GetCommitSHA1Result{
				"actions/checkout/v2": {SHA: sha},
			},
			line: "  - uses: actions/checkout@v2",
			exp:  "  - uses: actions/checkout@" + sha + " # 2.11.5",
		},
		{
			name: "only non-version tags - error because SHA can't be cross-verified",
			tags: []*github.RepositoryTag{
				{
					Name:   new("latest"),
					Commit: &github.Commit{SHA: new(sha)},
				},
				{
					Name:   new("nightly"),
					Commit: &github.Commit{SHA: new(sha)},
				},
			},
			line:    "  - uses: actions/checkout@" + sha,
			wantErr: ErrMissingVersionComment,
		},
		{
			name:    "no tags at all - error because SHA can't be cross-verified",
			tags:    []*github.RepositoryTag{},
			line:    "  - uses: actions/checkout@" + sha,
			wantErr: ErrMissingVersionComment,
		},
		{
			name: "tag exists but for a different SHA - error",
			tags: []*github.RepositoryTag{
				{
					Name:   new("v1.0.0"),
					Commit: &github.Commit{SHA: new("0000000000000000000000000000000000000000")},
				},
			},
			line:    "  - uses: actions/checkout@" + sha,
			wantErr: ErrMissingVersionComment,
		},
	}
	logger := slog.New(slog.DiscardHandler)
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			commits := d.commits
			if commits == nil {
				commits = map[string]*github.GetCommitSHA1Result{}
			}
			ctrl := New(&github.RepositoriesServiceImpl{
				Tags: map[string]*github.ListTagsResult{
					"actions/checkout/0": {
						Tags:     d.tags,
						Response: &github.Response{},
					},
				},
				Releases: map[string]*github.ListReleasesResult{
					"actions/checkout/0": {
						Releases: []*github.RepositoryRelease{},
						Response: &github.Response{},
					},
				},
				Commits: commits,
			}, nil, fs, &config.Config{
				Separator: " # ",
			}, &ParamRun{})
			line, err := ctrl.parseLine(t.Context(), logger, d.line)
			if d.wantErr != nil {
				if !errors.Is(err, d.wantErr) {
					t.Fatalf("wanted error %v, got %v", d.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if line != d.exp {
				t.Fatalf("wanted %s, got %s", d.exp, line)
			}
		})
	}
}

func TestController_parseLine_noAPI(t *testing.T) {
	t.Parallel()
	sha := "8e5e7e5ab8b370d6c329ec480221332ada57f0ab"
	data := []struct {
		name    string
		line    string
		wantErr error
	}{
		{
			name: "pinned SHA with version comment is accepted",
			line: "  - uses: actions/checkout@" + sha + " # v3.5.2",
		},
		{
			name:    "pinned SHA without version comment is rejected",
			line:    "  - uses: actions/checkout@" + sha,
			wantErr: ErrMissingVersionComment,
		},
		{
			name:    "tag reference is rejected (can't pin without API)",
			line:    "  - uses: actions/checkout@v3",
			wantErr: ErrCantPinned,
		},
	}
	logger := slog.New(slog.DiscardHandler)
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			ctrl := New(nil, nil, fs, &config.Config{
				Separator: " # ",
			}, &ParamRun{NoAPI: true})
			_, err := ctrl.parseLine(t.Context(), logger, d.line)
			if d.wantErr != nil {
				if !errors.Is(err, d.wantErr) {
					t.Fatalf("wanted error %v, got %v", d.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func Test_patchLine(t *testing.T) {
	t.Parallel()
	data := []struct {
		name    string
		tag     string
		version string
		action  *Action
		exp     string
	}{
		{
			name: "checkout v3",
			exp:  "  - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v3.5.2",
			action: &Action{
				Uses:                    "  - uses: ",
				Name:                    "actions/checkout",
				Version:                 "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionCommentSeparator: " # ",
				VersionComment:          "v3",
			},
			version: "ee0669bd1cc54295c223e0bb666b733df41de1c5",
			tag:     "v3.5.2",
		},
		{
			name: "checkout v2",
			exp:  "  uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.17.0",
			action: &Action{
				Uses:    "  uses: ",
				Name:    "actions/checkout",
				Version: "v2",
			},
			version: "ee0669bd1cc54295c223e0bb666b733df41de1c5",
			tag:     "v2.17.0",
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			cfg := &config.Config{
				Separator: " # ",
			}
			ctrl := New(nil, nil, fs, cfg, &ParamRun{})
			line := ctrl.patchLine(d.action, d.version, d.tag)
			if line != d.exp {
				t.Fatalf(`wanted %s, got %s`, d.exp, line)
			}
		})
	}
}

func Test_patchLine_customSeparator(t *testing.T) {
	t.Parallel()
	data := []struct {
		name      string
		tag       string
		version   string
		action    *Action
		separator string
		exp       string
	}{
		{
			name: "custom separator",
			exp:  "  - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5  # v3.5.2",
			action: &Action{
				Uses:    "  - uses: ",
				Name:    "actions/checkout",
				Version: "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
			},
			version:   "ee0669bd1cc54295c223e0bb666b733df41de1c5",
			tag:       "v3.5.2",
			separator: "  # ",
		},
		{
			name: "existing separator preserved",
			exp:  "  uses: actions/setup-go@abc123 # tag=v4.0.0",
			action: &Action{
				Uses:                    "  uses: ",
				Name:                    "actions/setup-go",
				Version:                 "v3",
				VersionCommentSeparator: " # tag=",
			},
			version:   "abc123",
			tag:       "v4.0.0",
			separator: "  # ",
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			cfg := &config.Config{Separator: d.separator}
			ctrl := New(nil, nil, fs, cfg, &ParamRun{})
			line := ctrl.patchLine(d.action, d.version, d.tag)
			if line != d.exp {
				t.Fatalf(`wanted %s, got %s`, d.exp, line)
			}
		})
	}
}

func Test_getVersionType(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name    string
		version string
		want    VersionType
	}{
		{
			name:    "empty string",
			version: "",
			want:    Empty,
		},
		{
			name:    "full commit SHA",
			version: "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
			want:    FullCommitSHA,
		},
		{
			name:    "semver with v prefix",
			version: "v1.2.3",
			want:    Semver,
		},
		{
			name:    "semver without v prefix",
			version: "1.2.3",
			want:    Semver,
		},
		{
			name:    "semver with prerelease",
			version: "v1.2.3-alpha",
			want:    Semver,
		},
		{
			name:    "semver with build metadata",
			version: "v1.2.3+build.1",
			want:    Semver,
		},
		{
			name:    "short semver v3",
			version: "v3",
			want:    Shortsemver,
		},
		{
			name:    "short semver v3.1",
			version: "v3.1",
			want:    Shortsemver,
		},
		{
			name:    "short semver without v prefix",
			version: "3",
			want:    Shortsemver,
		},
		{
			name:    "short semver minor without v",
			version: "3.1",
			want:    Shortsemver,
		},
		{
			name:    "branch name main",
			version: "main",
			want:    Other,
		},
		{
			name:    "branch name master",
			version: "master",
			want:    Other,
		},
		{
			name:    "short SHA",
			version: "abc1234",
			want:    Other,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := getVersionType(tt.version); got != tt.want {
				t.Errorf("getVersionType(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func Test_compareVersion(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name           string
		currentVersion string
		newVersion     string
		want           bool
	}{
		{
			name:           "new version is greater",
			currentVersion: "v1.0.0",
			newVersion:     "v2.0.0",
			want:           true,
		},
		{
			name:           "new version is less",
			currentVersion: "v2.0.0",
			newVersion:     "v1.0.0",
			want:           false,
		},
		{
			name:           "versions are equal",
			currentVersion: "v1.0.0",
			newVersion:     "v1.0.0",
			want:           false,
		},
		{
			name:           "minor version is greater",
			currentVersion: "v1.0.0",
			newVersion:     "v1.1.0",
			want:           true,
		},
		{
			name:           "patch version is greater",
			currentVersion: "v1.0.0",
			newVersion:     "v1.0.1",
			want:           true,
		},
		{
			name:           "invalid current version - string comparison",
			currentVersion: "main",
			newVersion:     "release",
			want:           true,
		},
		{
			name:           "invalid new version - string comparison",
			currentVersion: "v1.0.0",
			newVersion:     "invalid",
			want:           false,
		},
		{
			name:           "both invalid - string comparison greater",
			currentVersion: "alpha",
			newVersion:     "beta",
			want:           true,
		},
		{
			name:           "both invalid - string comparison less",
			currentVersion: "beta",
			newVersion:     "alpha",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := compareVersion(tt.currentVersion, tt.newVersion); got != tt.want {
				t.Errorf("compareVersion(%q, %q) = %v, want %v", tt.currentVersion, tt.newVersion, got, tt.want)
			}
		})
	}
}

func TestController_shouldSkipAction(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name   string
		cfg    *config.Config
		param  *ParamRun
		action *Action
		want   bool
	}{
		{
			name:  "no filters - should not skip",
			cfg:   &config.Config{},
			param: &ParamRun{},
			action: &Action{
				Name:    "actions/checkout",
				Version: "v3",
			},
			want: false,
		},
		{
			name: "excluded by exclude pattern",
			cfg:  &config.Config{},
			param: &ParamRun{
				Excludes: []*regexp.Regexp{regexp.MustCompile(`^actions/.*`)},
			},
			action: &Action{
				Name:    "actions/checkout",
				Version: "v3",
			},
			want: true,
		},
		{
			name: "not excluded by non-matching exclude pattern",
			cfg:  &config.Config{},
			param: &ParamRun{
				Excludes: []*regexp.Regexp{regexp.MustCompile(`^other/.*`)},
			},
			action: &Action{
				Name:    "actions/checkout",
				Version: "v3",
			},
			want: false,
		},
		{
			name: "included by include pattern",
			cfg:  &config.Config{},
			param: &ParamRun{
				Includes: []*regexp.Regexp{regexp.MustCompile(`^actions/.*`)},
			},
			action: &Action{
				Name:    "actions/checkout",
				Version: "v3",
			},
			want: false,
		},
		{
			name: "excluded by non-matching include pattern",
			cfg:  &config.Config{},
			param: &ParamRun{
				Includes: []*regexp.Regexp{regexp.MustCompile(`^other/.*`)},
			},
			action: &Action{
				Name:    "actions/checkout",
				Version: "v3",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			ctrl := New(nil, nil, fs, tt.cfg, tt.param)
			logger := slog.New(slog.DiscardHandler)

			if got := ctrl.shouldSkipAction(logger, tt.action); got != tt.want {
				t.Errorf("shouldSkipAction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_parseActionName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		action        *Action
		want          bool
		wantRepoOwner string
		wantRepoName  string
	}{
		{
			name: "valid action name",
			action: &Action{
				Name: "actions/checkout",
			},
			want:          true,
			wantRepoOwner: "actions",
			wantRepoName:  "checkout",
		},
		{
			name: "action with path",
			action: &Action{
				Name: "owner/repo/path/to/action",
			},
			want:          true,
			wantRepoOwner: "owner",
			wantRepoName:  "repo",
		},
		{
			name: "single component name",
			action: &Action{
				Name: "localaction",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			ctrl := New(nil, nil, fs, &config.Config{}, &ParamRun{})

			got := ctrl.parseActionName(tt.action)

			if got != tt.want {
				t.Errorf("parseActionName() = %v, want %v", got, tt.want)
			}
			if tt.want {
				if tt.action.RepoOwner != tt.wantRepoOwner {
					t.Errorf("RepoOwner = %v, want %v", tt.action.RepoOwner, tt.wantRepoOwner)
				}
				if tt.action.RepoName != tt.wantRepoName {
					t.Errorf("RepoName = %v, want %v", tt.action.RepoName, tt.wantRepoName)
				}
			}
		})
	}
}

func TestController_excludeAction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		excludes   []*regexp.Regexp
		actionName string
		want       bool
	}{
		{
			name:       "no excludes",
			excludes:   nil,
			actionName: "actions/checkout",
			want:       false,
		},
		{
			name:       "empty excludes",
			excludes:   []*regexp.Regexp{},
			actionName: "actions/checkout",
			want:       false,
		},
		{
			name:       "matching exclude pattern",
			excludes:   []*regexp.Regexp{regexp.MustCompile(`actions/checkout`)},
			actionName: "actions/checkout",
			want:       true,
		},
		{
			name:       "non-matching exclude pattern",
			excludes:   []*regexp.Regexp{regexp.MustCompile(`other/action`)},
			actionName: "actions/checkout",
			want:       false,
		},
		{
			name: "multiple excludes - one matches",
			excludes: []*regexp.Regexp{
				regexp.MustCompile(`other/action`),
				regexp.MustCompile(`actions/.*`),
			},
			actionName: "actions/checkout",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			ctrl := New(nil, nil, fs, &config.Config{}, &ParamRun{Excludes: tt.excludes})

			if got := ctrl.excludeAction(tt.actionName); got != tt.want {
				t.Errorf("excludeAction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_excludeByIncludes(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name       string
		includes   []*regexp.Regexp
		actionName string
		want       bool
	}{
		{
			name:       "no includes - not excluded",
			includes:   nil,
			actionName: "actions/checkout",
			want:       false,
		},
		{
			name:       "empty includes - not excluded",
			includes:   []*regexp.Regexp{},
			actionName: "actions/checkout",
			want:       false,
		},
		{
			name:       "matching include - not excluded",
			includes:   []*regexp.Regexp{regexp.MustCompile(`actions/.*`)},
			actionName: "actions/checkout",
			want:       false,
		},
		{
			name:       "non-matching include - excluded",
			includes:   []*regexp.Regexp{regexp.MustCompile(`other/.*`)},
			actionName: "actions/checkout",
			want:       true,
		},
		{
			name: "multiple includes - one matches - not excluded",
			includes: []*regexp.Regexp{
				regexp.MustCompile(`other/.*`),
				regexp.MustCompile(`actions/.*`),
			},
			actionName: "actions/checkout",
			want:       false,
		},
		{
			name: "multiple includes - none match - excluded",
			includes: []*regexp.Regexp{
				regexp.MustCompile(`other/.*`),
				regexp.MustCompile(`another/.*`),
			},
			actionName: "actions/checkout",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			ctrl := New(nil, nil, fs, &config.Config{}, &ParamRun{Includes: tt.includes})

			if got := ctrl.excludeByIncludes(tt.actionName); got != tt.want {
				t.Errorf("excludeByIncludes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_parseLine_branchToTag(t *testing.T) { //nolint:funlen
	t.Parallel()
	// Tag/release setup used by all sub-tests: actions/checkout has a stable
	// v3.5.2 release plus a v4.0.0-rc.1 pre-release. actions/no-stable has only
	// a pre-release. actions/no-tag has neither.
	tagsCheckout := &github.ListTagsResult{
		Tags: []*github.RepositoryTag{
			{Name: new("v3.5.2"), Commit: &github.Commit{SHA: new("8e5e7e5ab8b370d6c329ec480221332ada57f0ab")}},
			{Name: new("v4.0.0-rc.1"), Commit: &github.Commit{SHA: new("rcrcrcrcrcrcrcrcrcrcrcrcrcrcrcrcrcrcrcrc")}},
		},
		Response: &github.Response{},
	}
	releasesCheckout := &github.ListReleasesResult{
		Releases: []*github.RepositoryRelease{
			{TagName: new("v3.5.2")},
			{TagName: new("v4.0.0-rc.1"), Prerelease: new(true)},
		},
		Response: &github.Response{},
	}
	tagsNoStable := &github.ListTagsResult{
		Tags: []*github.RepositoryTag{
			{Name: new("v1.0.0-beta"), Commit: &github.Commit{SHA: new("bebebebebebebebebebebebebebebebebebebebe")}},
		},
		Response: &github.Response{},
	}
	releasesNoStable := &github.ListReleasesResult{
		Releases: []*github.RepositoryRelease{
			{TagName: new("v1.0.0-beta"), Prerelease: new(true)},
		},
		Response: &github.Response{},
	}
	tagsNoTag := &github.ListTagsResult{Tags: []*github.RepositoryTag{}, Response: &github.Response{}}
	releasesNoTag := &github.ListReleasesResult{Releases: []*github.RepositoryRelease{}, Response: &github.Response{}}

	// actions/min-age has v2.0.0 (recent, 1 day ago) and v1.0.0 (older, 30 days ago).
	// With --min-age 7, v2.0.0 must be skipped and v1.0.0 chosen.
	now := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	tagsMinAge := &github.ListTagsResult{
		Tags: []*github.RepositoryTag{
			{Name: new("v2.0.0"), Commit: &github.Commit{SHA: new("2222222222222222222222222222222222222222")}},
			{Name: new("v1.0.0"), Commit: &github.Commit{SHA: new("1111111111111111111111111111111111111111")}},
		},
		Response: &github.Response{},
	}
	releasesMinAge := &github.ListReleasesResult{
		Releases: []*github.RepositoryRelease{
			{TagName: new("v2.0.0"), PublishedAt: &github.Timestamp{Time: now.AddDate(0, 0, -1)}},
			{TagName: new("v1.0.0"), PublishedAt: &github.Timestamp{Time: now.AddDate(0, 0, -30)}},
		},
		Response: &github.Response{},
	}

	commits := map[string]*github.GetCommitSHA1Result{
		"actions/checkout/v3.5.2":       {SHA: "8e5e7e5ab8b370d6c329ec480221332ada57f0ab"},
		"actions/no-stable/v1.0.0-beta": {SHA: "bebebebebebebebebebebebebebebebebebebebe"},
		"actions/min-age/v1.0.0":        {SHA: "1111111111111111111111111111111111111111"},
	}

	data := []struct {
		name        string
		line        string
		branchToTag []*regexp.Regexp
		minAge      int
		exp         string
		isErr       bool
	}{
		{
			name:        "main matches and is converted to latest stable tag",
			line:        "  - uses: actions/checkout@main",
			branchToTag: []*regexp.Regexp{regexp.MustCompile(`^main$`)},
			exp:         "  - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3.5.2",
		},
		{
			name:        "regex unmatched still errors",
			line:        "  - uses: actions/checkout@main",
			branchToTag: []*regexp.Regexp{regexp.MustCompile(`^master$`)},
			isErr:       true,
		},
		{
			name:        "no branch-to-tag configured still errors",
			line:        "  - uses: actions/checkout@main",
			branchToTag: nil,
			isErr:       true,
		},
		{
			name:        "falls back to pre-release when no stable tag exists",
			line:        "  - uses: actions/no-stable@develop",
			branchToTag: []*regexp.Regexp{regexp.MustCompile(`^develop$`)},
			exp:         "  - uses: actions/no-stable@bebebebebebebebebebebebebebebebebebebebe # v1.0.0-beta",
		},
		{
			name:        "errors when action has no tag at all",
			line:        "  - uses: actions/no-tag@main",
			branchToTag: []*regexp.Regexp{regexp.MustCompile(`^main$`)},
			isErr:       true,
		},
		{
			name:        "semver line is unaffected by branch-to-tag",
			line:        "  - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3",
			branchToTag: []*regexp.Regexp{regexp.MustCompile(`.*`)},
			exp:         "  - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3.5.2",
		},
		{
			name:        "min-age skips recent stable tag",
			line:        "  - uses: actions/min-age@main",
			branchToTag: []*regexp.Regexp{regexp.MustCompile(`^main$`)},
			minAge:      7,
			exp:         "  - uses: actions/min-age@1111111111111111111111111111111111111111 # v1.0.0",
		},
	}
	logger := slog.New(slog.DiscardHandler)
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			ctrl := New(&github.RepositoriesServiceImpl{
				Tags: map[string]*github.ListTagsResult{
					"actions/checkout/0":  tagsCheckout,
					"actions/no-stable/0": tagsNoStable,
					"actions/no-tag/0":    tagsNoTag,
					"actions/min-age/0":   tagsMinAge,
				},
				Releases: map[string]*github.ListReleasesResult{
					"actions/checkout/0":  releasesCheckout,
					"actions/no-stable/0": releasesNoStable,
					"actions/no-tag/0":    releasesNoTag,
					"actions/min-age/0":   releasesMinAge,
				},
				Commits: commits,
			}, nil, fs, &config.Config{Separator: " # "}, &ParamRun{
				BranchToTags: d.branchToTag,
				MinAge:       d.minAge,
				Now:          now,
			})
			line, err := ctrl.parseLine(t.Context(), logger, d.line)
			if err != nil {
				if d.isErr {
					return
				}
				t.Fatal(err)
			}
			if d.isErr {
				t.Fatalf("expected error, got line %q", line)
			}
			if line != d.exp {
				t.Fatalf("wanted %s, got %s", d.exp, line)
			}
		})
	}
}

// TestController_parseLine_update_ruleMinAge verifies that rules[].min_age
// overrides the global min_age.value when selecting the update target during
// `pinact run -u`. With the global cutoff (3 days) v2.0.0 would be filtered out,
// but the rule sets min_age to 0 so the recent release must be picked.
func TestController_parseLine_update_ruleMinAge(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)
	tags := &github.ListTagsResult{
		Tags: []*github.RepositoryTag{
			{Name: new("v2.0.0"), Commit: &github.Commit{SHA: new("2222222222222222222222222222222222222222")}},
			{Name: new("v1.0.0"), Commit: &github.Commit{SHA: new("1111111111111111111111111111111111111111")}},
		},
		Response: &github.Response{},
	}
	releases := &github.ListReleasesResult{
		Releases: []*github.RepositoryRelease{
			{TagName: new("v2.0.0"), PublishedAt: &github.Timestamp{Time: now.AddDate(0, 0, -1)}},
			{TagName: new("v1.0.0"), PublishedAt: &github.Timestamp{Time: now.AddDate(0, 0, -30)}},
		},
		Response: &github.Response{},
	}
	commits := map[string]*github.GetCommitSHA1Result{
		"aquaproj/example/v2.0.0": {SHA: "2222222222222222222222222222222222222222"},
	}

	three := 3
	zero := 0
	cfg := &config.Config{
		Separator: " # ",
		MinAge:    &config.MinAge{Value: &three},
		Rules: []*config.Rule{
			{
				MinAge: &zero,
				Conditions: []*config.Condition{
					{Expr: `ActionRepoOwner == "aquaproj"`},
				},
			},
		},
	}
	for i, r := range cfg.Rules {
		if err := r.Init(); err != nil {
			t.Fatalf("init rules[%d]: %v", i, err)
		}
	}

	fs := afero.NewMemMapFs()
	ctrl := New(&github.RepositoriesServiceImpl{
		Tags:     map[string]*github.ListTagsResult{"aquaproj/example/0": tags},
		Releases: map[string]*github.ListReleasesResult{"aquaproj/example/0": releases},
		Commits:  commits,
	}, nil, fs, cfg, &ParamRun{
		Update: true,
		Now:    now,
	})
	logger := slog.New(slog.DiscardHandler)

	line := "  - uses: aquaproj/example@1111111111111111111111111111111111111111 # v1.0.0"
	got, err := ctrl.parseLine(t.Context(), logger, line)
	if err != nil {
		t.Fatal(err)
	}
	want := "  - uses: aquaproj/example@2222222222222222222222222222222222222222 # v2.0.0"
	if got != want {
		t.Fatalf("rules[].min_age=0 should override cooldown:\n  want %s\n  got  %s", want, got)
	}
}

// TestController_checkSHAMinAge_boundary verifies that the passive -min-age
// check returns ErrMinAge when the commit is younger than the cutoff and
// returns nil when it is older. The mock GitService is reused from
// github_internal_test.go.
func TestController_checkSHAMinAge_boundary(t *testing.T) { //nolint:funlen
	t.Parallel()
	now := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)
	cutoff := now.AddDate(0, 0, -7) // 2026-05-09

	tests := []struct {
		name        string
		committedAt time.Time
		minAge      int
		wantErr     bool
	}{
		{
			name:        "committed 1 day before cutoff -> ok",
			committedAt: cutoff.AddDate(0, 0, -1),
			minAge:      7,
			wantErr:     false,
		},
		{
			name:        "committed exactly at cutoff -> ok",
			committedAt: cutoff,
			minAge:      7,
			wantErr:     false,
		},
		{
			name:        "committed 1 day after cutoff -> violation",
			committedAt: cutoff.AddDate(0, 0, 1),
			minAge:      7,
			wantErr:     true,
		},
		{
			name:        "minAge=0 disables the check",
			committedAt: now,
			minAge:      0,
			wantErr:     false,
		},
	}
	logger := slog.New(slog.DiscardHandler)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			committedAt := tt.committedAt
			gs := &mockGitService{
				getCommitFunc: func(_ context.Context, _, _, _ string) (*github.Commit, *github.Response, error) {
					return &github.Commit{
						Committer: &github.CommitAuthor{
							Date: &github.Timestamp{Time: committedAt},
						},
					}, &github.Response{}, nil
				},
			}
			ctrl := New(nil, gs, fs, &config.Config{}, &ParamRun{
				Now:          now,
				VerifyMinAge: true, // enable the passive check for this test
			})
			err := ctrl.checkSHAMinAge(t.Context(), logger, "owner", "repo", "deadbeef", tt.minAge)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected ErrMinAge, got nil")
				}
				if !errors.Is(err, ErrMinAge) {
					t.Fatalf("expected ErrMinAge, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// TestController_checkSHAMinAge_disabledByDefault verifies that the passive
// audit is skipped when neither ParamRun.VerifyMinAge nor cfg.VerifyMinAge is
// set, even if the commit would otherwise violate the cutoff.
func TestController_checkSHAMinAge_disabledByDefault(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC)
	logger := slog.New(slog.DiscardHandler)
	called := false
	gs := &mockGitService{
		getCommitFunc: func(_ context.Context, _, _, _ string) (*github.Commit, *github.Response, error) {
			called = true
			// Return a commit far younger than the cutoff so any check that
			// runs would surface ErrMinAge.
			return &github.Commit{
				Committer: &github.CommitAuthor{
					Date: &github.Timestamp{Time: now},
				},
			}, &github.Response{}, nil
		},
	}
	ctrl := New(nil, gs, afero.NewMemMapFs(), &config.Config{}, &ParamRun{
		Now: now,
		// VerifyMinAge intentionally left false
	})
	if err := ctrl.checkSHAMinAge(t.Context(), logger, "owner", "repo", "deadbeef", 7); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("checkSHAMinAge called GetCommit even though VerifyMinAge is false")
	}
}

// TestController_effectiveMinAge verifies the CLI/env > rules > config
// precedence for resolving the per-action min-age threshold. PINACT_MIN_AGE
// is wired into param.MinAge via urfave Sources, so it shares the slot with
// the CLI -min-age flag and the "cliMinAge" column models either source.
func TestController_effectiveMinAge(t *testing.T) {
	t.Parallel()
	zero := 0
	five := 5
	seven := 7
	tests := []struct {
		name         string
		cliMinAge    int
		topLevelMin  *int
		ruleOverride *int
		want         int
	}{
		{name: "CLI / env flag wins over rules / top-level", cliMinAge: 14, topLevelMin: &seven, ruleOverride: &five, want: 14},
		{name: "rule overrides top-level when CLI unset", cliMinAge: 0, topLevelMin: &seven, ruleOverride: &five, want: 5},
		{name: "rule min_age 0 disables check when CLI unset", cliMinAge: 0, topLevelMin: &seven, ruleOverride: &zero, want: 0},
		{name: "top-level applies when no rule matched", cliMinAge: 0, topLevelMin: &seven, ruleOverride: nil, want: 7},
		{name: "default 0 when nothing is set", cliMinAge: 0, topLevelMin: nil, ruleOverride: nil, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := &Controller{
				cfg:   &config.Config{MinAge: &config.MinAge{Value: tt.topLevelMin}},
				param: &ParamRun{MinAge: tt.cliMinAge},
			}
			got := ctrl.effectiveMinAge(&config.Resolved{MinAge: tt.ruleOverride})
			if got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

// TestController_effectiveKeepMajor verifies the rules > CLI > config
// precedence for the per-action keep-major decision. Rules win over the CLI
// so an explicit per-action opt-out (rules[].keep_major: false) is honored
// even when the user passes --keep-major globally.
func TestController_effectiveKeepMajor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		cliKeepMajor  bool
		cfgKeepMajor  *bool
		ruleKeepMajor *bool
		want          bool
	}{
		{name: "default false when nothing is set", cliKeepMajor: false, cfgKeepMajor: nil, ruleKeepMajor: nil, want: false},
		{name: "CLI true with no rule and no cfg", cliKeepMajor: true, cfgKeepMajor: nil, ruleKeepMajor: nil, want: true},
		{name: "cfg true with no CLI and no rule", cliKeepMajor: false, cfgKeepMajor: new(true), ruleKeepMajor: nil, want: true},
		{name: "rule true overrides cfg/CLI false", cliKeepMajor: false, cfgKeepMajor: new(false), ruleKeepMajor: new(true), want: true},
		{name: "rule false overrides CLI true (per-action opt-out)", cliKeepMajor: true, cfgKeepMajor: nil, ruleKeepMajor: new(false), want: false},
		{name: "rule false overrides cfg true (per-action opt-out)", cliKeepMajor: false, cfgKeepMajor: new(true), ruleKeepMajor: new(false), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := &Controller{
				cfg:   &config.Config{KeepMajor: tt.cfgKeepMajor},
				param: &ParamRun{KeepMajor: tt.cliKeepMajor},
			}
			got := ctrl.effectiveKeepMajor(&config.Resolved{KeepMajor: tt.ruleKeepMajor})
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestController_parseLine_update_keepMajor verifies that --keep-major
// restricts -u to releases within the same major version as the current pin's
// version comment. The repository advertises v6.0.0 as the latest, but the
// action is pinned at v4.3.1 so the upgrade target must stay at v4.x.
func TestController_parseLine_update_keepMajor(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	tags := &github.ListTagsResult{
		Tags: []*github.RepositoryTag{
			{Name: new("v6.0.0"), Commit: &github.Commit{SHA: new("6666666666666666666666666666666666666666")}},
			{Name: new("v4.4.0"), Commit: &github.Commit{SHA: new("4444444444444444444444444444444444444444")}},
			{Name: new("v4.3.1"), Commit: &github.Commit{SHA: new("3333333333333333333333333333333333333333")}},
		},
		Response: &github.Response{},
	}
	releases := &github.ListReleasesResult{
		Releases: []*github.RepositoryRelease{
			{TagName: new("v6.0.0"), PublishedAt: &github.Timestamp{Time: now.AddDate(0, 0, -10)}},
			{TagName: new("v4.4.0"), PublishedAt: &github.Timestamp{Time: now.AddDate(0, 0, -20)}},
			{TagName: new("v4.3.1"), PublishedAt: &github.Timestamp{Time: now.AddDate(0, 0, -90)}},
		},
		Response: &github.Response{},
	}
	commits := map[string]*github.GetCommitSHA1Result{
		"aquaproj/example/v4.4.0": {SHA: "4444444444444444444444444444444444444444"},
	}
	cfg := &config.Config{Separator: " # "}
	fs := afero.NewMemMapFs()
	ctrl := New(&github.RepositoriesServiceImpl{
		Tags:     map[string]*github.ListTagsResult{"aquaproj/example/0": tags},
		Releases: map[string]*github.ListReleasesResult{"aquaproj/example/0": releases},
		Commits:  commits,
	}, nil, fs, cfg, &ParamRun{
		Update:    true,
		KeepMajor: true,
		Now:       now,
	})
	logger := slog.New(slog.DiscardHandler)

	line := "  - uses: aquaproj/example@3333333333333333333333333333333333333333 # v4.3.1"
	got, err := ctrl.parseLine(t.Context(), logger, line)
	if err != nil {
		t.Fatal(err)
	}
	want := "  - uses: aquaproj/example@4444444444444444444444444444444444444444 # v4.4.0"
	if got != want {
		t.Fatalf("--keep-major must keep the upgrade within v4.x:\n  want %s\n  got  %s", want, got)
	}
}

// TestController_parseLine_update_ruleKeepMajor verifies that a per-action
// rules[].keep_major override takes effect even when the global config and
// CLI both leave keep-major off. Without the rule, -u would jump to v6.0.0.
func TestController_parseLine_update_ruleKeepMajor(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 5, 24, 0, 0, 0, 0, time.UTC)
	tags := &github.ListTagsResult{
		Tags: []*github.RepositoryTag{
			{Name: new("v6.0.0"), Commit: &github.Commit{SHA: new("6666666666666666666666666666666666666666")}},
			{Name: new("v4.4.0"), Commit: &github.Commit{SHA: new("4444444444444444444444444444444444444444")}},
		},
		Response: &github.Response{},
	}
	releases := &github.ListReleasesResult{
		Releases: []*github.RepositoryRelease{
			{TagName: new("v6.0.0"), PublishedAt: &github.Timestamp{Time: now.AddDate(0, 0, -10)}},
			{TagName: new("v4.4.0"), PublishedAt: &github.Timestamp{Time: now.AddDate(0, 0, -20)}},
		},
		Response: &github.Response{},
	}
	commits := map[string]*github.GetCommitSHA1Result{
		"aquaproj/example/v4.4.0": {SHA: "4444444444444444444444444444444444444444"},
	}
	cfg := &config.Config{
		Separator: " # ",
		Rules: []*config.Rule{
			{
				KeepMajor: new(true),
				Conditions: []*config.Condition{
					{Expr: `ActionRepoOwner == "aquaproj"`},
				},
			},
		},
	}
	for i, r := range cfg.Rules {
		if err := r.Init(); err != nil {
			t.Fatalf("init rules[%d]: %v", i, err)
		}
	}
	fs := afero.NewMemMapFs()
	ctrl := New(&github.RepositoriesServiceImpl{
		Tags:     map[string]*github.ListTagsResult{"aquaproj/example/0": tags},
		Releases: map[string]*github.ListReleasesResult{"aquaproj/example/0": releases},
		Commits:  commits,
	}, nil, fs, cfg, &ParamRun{
		Update: true,
		Now:    now,
	})
	logger := slog.New(slog.DiscardHandler)

	line := "  - uses: aquaproj/example@3333333333333333333333333333333333333333 # v4.3.1"
	got, err := ctrl.parseLine(t.Context(), logger, line)
	if err != nil {
		t.Fatal(err)
	}
	want := "  - uses: aquaproj/example@4444444444444444444444444444444444444444 # v4.4.0"
	if got != want {
		t.Fatalf("rules[].keep_major=true must keep the upgrade within v4.x:\n  want %s\n  got  %s", want, got)
	}
}

// TestController_minAgeFallback verifies the CLI/env > config.min_age.value
// precedence used by contexts without rule resolution (the -update cooldown
// filter inside getLatestVersionWithStable).
func TestController_minAgeFallback(t *testing.T) {
	t.Parallel()
	seven := 7
	sixty := 60
	tests := []struct {
		name        string
		cliMinAge   int
		topLevelMin *int
		want        int
	}{
		{name: "CLI / env wins", cliMinAge: 14, topLevelMin: &seven, want: 14},
		{name: "config applies when CLI / env unset", cliMinAge: 0, topLevelMin: &sixty, want: 60},
		{name: "default 0 when nothing is set", cliMinAge: 0, topLevelMin: nil, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := &Controller{
				cfg:   &config.Config{MinAge: &config.MinAge{Value: tt.topLevelMin}},
				param: &ParamRun{MinAge: tt.cliMinAge},
			}
			if got := ctrl.minAgeFallback(); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}
