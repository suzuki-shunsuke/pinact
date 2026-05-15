package run

import (
	"log/slog"
	"regexp"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
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
			}, nil, nil, fs, &config.Config{
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
			ctrl := New(nil, nil, nil, fs, cfg, &ParamRun{})
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
			ctrl := New(nil, nil, nil, fs, cfg, &ParamRun{})
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
			ctrl := New(nil, nil, nil, fs, tt.cfg, tt.param)
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
			ctrl := New(nil, nil, nil, fs, &config.Config{}, &ParamRun{})

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
			ctrl := New(nil, nil, nil, fs, &config.Config{}, &ParamRun{Excludes: tt.excludes})

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
			ctrl := New(nil, nil, nil, fs, &config.Config{}, &ParamRun{Includes: tt.includes})

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
			}, nil, nil, fs, &config.Config{Separator: " # "}, &ParamRun{
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
