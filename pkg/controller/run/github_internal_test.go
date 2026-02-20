package run

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
)

func Test_compare(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name              string
		latestSemver      *version.Version
		latestVersion     string
		tag               string
		wantSemver        string
		wantLatestVersion string
		wantErr           bool
	}{
		{
			name:              "new semver is greater than current semver",
			latestSemver:      version.Must(version.NewVersion("1.0.0")),
			latestVersion:     "",
			tag:               "2.0.0",
			wantSemver:        "2.0.0",
			wantLatestVersion: "",
			wantErr:           false,
		},
		{
			name:              "new semver is less than current semver",
			latestSemver:      version.Must(version.NewVersion("2.0.0")),
			latestVersion:     "",
			tag:               "1.0.0",
			wantSemver:        "2.0.0",
			wantLatestVersion: "",
			wantErr:           false,
		},
		{
			name:              "new semver equals current semver",
			latestSemver:      version.Must(version.NewVersion("1.0.0")),
			latestVersion:     "",
			tag:               "1.0.0",
			wantSemver:        "1.0.0",
			wantLatestVersion: "",
			wantErr:           false,
		},
		{
			name:              "first semver with nil latest",
			latestSemver:      nil,
			latestVersion:     "",
			tag:               "1.2.3",
			wantSemver:        "1.2.3",
			wantLatestVersion: "",
			wantErr:           false,
		},
		{
			name:              "semver with v prefix",
			latestSemver:      nil,
			latestVersion:     "",
			tag:               "v1.2.3",
			wantSemver:        "v1.2.3",
			wantLatestVersion: "",
			wantErr:           false,
		},
		{
			name:              "invalid semver with greater string comparison",
			latestSemver:      nil,
			latestVersion:     "main",
			tag:               "release",
			wantSemver:        "",
			wantLatestVersion: "release",
			wantErr:           true,
		},
		{
			name:              "invalid semver with lesser string comparison",
			latestSemver:      nil,
			latestVersion:     "release",
			tag:               "main",
			wantSemver:        "",
			wantLatestVersion: "release",
			wantErr:           true,
		},
		{
			name:              "invalid semver as first tag",
			latestSemver:      nil,
			latestVersion:     "",
			tag:               "not-a-version",
			wantSemver:        "",
			wantLatestVersion: "not-a-version",
			wantErr:           true,
		},
		{
			name:              "invalid tag with existing semver",
			latestSemver:      version.Must(version.NewVersion("1.0.0")),
			latestVersion:     "",
			tag:               "invalid",
			wantSemver:        "1.0.0",
			wantLatestVersion: "invalid",
			wantErr:           true,
		},
		{
			name:              "compare with prerelease versions",
			latestSemver:      version.Must(version.NewVersion("1.0.0-alpha")),
			latestVersion:     "",
			tag:               "1.0.0",
			wantSemver:        "1.0.0",
			wantLatestVersion: "",
			wantErr:           false,
		},
		{
			name:              "compare with build metadata",
			latestSemver:      version.Must(version.NewVersion("1.0.0+build.1")),
			latestVersion:     "",
			tag:               "1.0.0+build.2",
			wantSemver:        "1.0.0+build.1",
			wantLatestVersion: "",
			wantErr:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotSemver, gotLatestVersion, err := compare(tt.latestSemver, tt.latestVersion, tt.tag)

			if (err != nil) != tt.wantErr {
				t.Errorf("compare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check semver result
			if tt.wantSemver == "" {
				if gotSemver != nil {
					t.Errorf("compare() gotSemver = %v, want nil", gotSemver)
				}
			} else {
				if gotSemver == nil {
					t.Errorf("compare() gotSemver = nil, want %v", tt.wantSemver)
				} else if gotSemver.Original() != tt.wantSemver {
					t.Errorf("compare() gotSemver = %v, want %v", gotSemver.Original(), tt.wantSemver)
				}
			}

			// Check latest version string result
			if gotLatestVersion != tt.wantLatestVersion {
				t.Errorf("compare() gotLatestVersion = %v, want %v", gotLatestVersion, tt.wantLatestVersion)
			}
		})
	}
}

// mockRepoService is a mock implementation of RepositoriesService for testing the underlying service
type mockRepoService struct {
	listReleasesFunc func(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error)
	listTagsFunc     func(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error)
}

func (m *mockRepoService) ListTags(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error) {
	if m.listTagsFunc != nil {
		return m.listTagsFunc(ctx, owner, repo, opts)
	}
	return nil, nil, errors.New("not implemented")
}

func (m *mockRepoService) GetCommitSHA1(_ context.Context, _, _, _, _ string) (string, *github.Response, error) {
	return "", nil, errors.New("not implemented")
}

func (m *mockRepoService) ListReleases(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error) {
	if m.listReleasesFunc != nil {
		return m.listReleasesFunc(ctx, owner, repo, opts)
	}
	return nil, nil, errors.New("not implemented")
}

func (m *mockRepoService) Get(_ context.Context, _, _ string) (*github.Repository, *github.Response, error) {
	return nil, nil, nil
}

// newTestRepoService creates a RepositoriesServiceImpl with the given mock for testing
func newTestRepoService(mock *mockRepoService) *github.RepositoriesServiceImpl {
	resolver := github.NewClientResolver(mock, nil, nil, nil, false)
	impl := &github.RepositoriesServiceImpl{
		Tags:     map[string]*github.ListTagsResult{},
		Releases: map[string]*github.ListReleasesResult{},
		Commits:  map[string]*github.GetCommitSHA1Result{},
	}
	impl.SetResolver(resolver)
	return impl
}

func TestController_getLatestVersionFromReleases(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name        string
		releases    []*github.RepositoryRelease
		listErr     error
		isStable    bool
		wantVersion string
		wantErr     bool
	}{
		{
			name: "single semver release",
			releases: []*github.RepositoryRelease{
				{TagName: new("v1.0.0")},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name: "multiple semver releases - returns highest",
			releases: []*github.RepositoryRelease{
				{TagName: new("v1.0.0")},
				{TagName: new("v2.0.0")},
				{TagName: new("v1.5.0")},
			},
			wantVersion: "v2.0.0",
			wantErr:     false,
		},
		{
			name: "mix of valid and invalid semver",
			releases: []*github.RepositoryRelease{
				{TagName: new("v1.0.0")},
				{TagName: new("not-a-version")},
				{TagName: new("v2.0.0")},
			},
			wantVersion: "v2.0.0",
			wantErr:     false,
		},
		{
			name: "only invalid versions - returns latest by string comparison",
			releases: []*github.RepositoryRelease{
				{TagName: new("main")},
				{TagName: new("release")},
				{TagName: new("develop")},
			},
			wantVersion: "release",
			wantErr:     false,
		},
		{
			name:        "no releases",
			releases:    []*github.RepositoryRelease{},
			wantVersion: "",
			wantErr:     false,
		},
		{
			name:        "nil releases",
			releases:    nil,
			wantVersion: "",
			wantErr:     false,
		},
		{
			name: "prerelease versions",
			releases: []*github.RepositoryRelease{
				{TagName: new("v1.0.0-alpha")},
				{TagName: new("v1.0.0-beta")},
				{TagName: new("v1.0.0")},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name: "build metadata versions",
			releases: []*github.RepositoryRelease{
				{TagName: new("v1.0.0+build.1")},
				{TagName: new("v1.0.0+build.2")},
				{TagName: new("v1.0.1")},
			},
			wantVersion: "v1.0.1",
			wantErr:     false,
		},
		{
			name: "releases with nil tag names",
			releases: []*github.RepositoryRelease{
				{TagName: nil},
				{TagName: new("v1.0.0")},
				{TagName: nil},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name:        "API error",
			releases:    nil,
			listErr:     errors.New("API error"),
			wantVersion: "",
			wantErr:     true,
		},
		{
			name: "empty tag name",
			releases: []*github.RepositoryRelease{
				{TagName: new("")},
				{TagName: new("v1.0.0")},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name: "stable version ignores prerelease when current is stable (issue #1095)",
			releases: []*github.RepositoryRelease{
				{TagName: new("v6-beta"), Prerelease: new(true)},
				{TagName: new("v5.0.0"), Prerelease: new(false)},
				{TagName: new("v4.3.0"), Prerelease: new(false)},
			},
			isStable:    true,
			wantVersion: "v5.0.0",
			wantErr:     false,
		},
		{
			name: "prerelease version can update to newer prerelease (issue #1095)",
			releases: []*github.RepositoryRelease{
				{TagName: new("v6-beta"), Prerelease: new(true)},
				{TagName: new("v5.0.0"), Prerelease: new(false)},
			},
			isStable:    false,
			wantVersion: "v6-beta",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockRepo := &mockRepoService{
				listReleasesFunc: func(_ context.Context, _, _ string, _ *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error) {
					return tt.releases, nil, tt.listErr
				},
			}

			c := &Controller{
				repositoriesService: newTestRepoService(mockRepo),
			}

			ctx := t.Context()
			logger := slog.New(slog.DiscardHandler)

			gotVersion, _, err := c.getLatestVersionFromReleases(ctx, logger, "owner", "repo", tt.isStable, time.Time{})

			if (err != nil) != tt.wantErr {
				t.Errorf("getLatestVersionFromReleases() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotVersion != tt.wantVersion {
				t.Errorf("getLatestVersionFromReleases() = %v, want %v", gotVersion, tt.wantVersion)
			}
		})
	}
}

func Test_isStableVersion(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{
			name:    "empty version",
			version: "",
			want:    false,
		},
		{
			name:    "stable semver with v prefix",
			version: "v1.2.3",
			want:    true,
		},
		{
			name:    "stable semver without v prefix",
			version: "1.2.3",
			want:    true,
		},
		{
			name:    "prerelease version alpha",
			version: "v1.2.3-alpha",
			want:    false,
		},
		{
			name:    "prerelease version beta",
			version: "v1.2.3-beta.1",
			want:    false,
		},
		{
			name:    "prerelease version rc",
			version: "v1.2.3-rc.1",
			want:    false,
		},
		{
			name:    "invalid version string",
			version: "not-a-version",
			want:    false,
		},
		{
			name:    "branch name",
			version: "main",
			want:    false,
		},
		{
			name:    "short version v3",
			version: "v3",
			want:    true,
		},
		{
			name:    "short version with prerelease",
			version: "v3-beta",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isStableVersion(tt.version); got != tt.want {
				t.Errorf("isStableVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func Test_checkTagCooldown(t *testing.T) { //nolint:funlen
	t.Parallel()
	now := time.Now()
	cutoff := now.AddDate(0, 0, -7) // 7 days ago

	tests := []struct {
		name       string
		gitService GitService
		sha        string
		cutoff     time.Time
		commitTime time.Time
		want       bool
	}{
		{
			name:       "zero cutoff - no cooldown check",
			gitService: nil,
			sha:        "abc123",
			cutoff:     time.Time{},
			want:       false,
		},
		{
			name:       "nil git service - no cooldown check",
			gitService: nil,
			sha:        "abc123",
			cutoff:     cutoff,
			want:       false,
		},
		{
			name:       "empty SHA - no cooldown check",
			gitService: &mockGitService{},
			sha:        "",
			cutoff:     cutoff,
			want:       false,
		},
		{
			name: "commit before cutoff - not skipped",
			gitService: &mockGitService{
				getCommitFunc: func(_ context.Context, _, _, _ string) (*github.Commit, *github.Response, error) {
					beforeCutoff := github.Timestamp{Time: cutoff.AddDate(0, 0, -1)}
					return &github.Commit{
						Committer: &github.CommitAuthor{
							Date: &beforeCutoff,
						},
					}, nil, nil
				},
			},
			sha:    "abc123",
			cutoff: cutoff,
			want:   false,
		},
		{
			name: "commit after cutoff - skipped",
			gitService: &mockGitService{
				getCommitFunc: func(_ context.Context, _, _, _ string) (*github.Commit, *github.Response, error) {
					afterCutoff := github.Timestamp{Time: cutoff.AddDate(0, 0, 1)}
					return &github.Commit{
						Committer: &github.CommitAuthor{
							Date: &afterCutoff,
						},
					}, nil, nil
				},
			},
			sha:    "abc123",
			cutoff: cutoff,
			want:   true,
		},
		{
			name: "API error - skipped",
			gitService: &mockGitService{
				getCommitFunc: func(_ context.Context, _, _, _ string) (*github.Commit, *github.Response, error) {
					return nil, nil, errors.New("API error")
				},
			},
			sha:    "abc123",
			cutoff: cutoff,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			logger := slog.New(slog.DiscardHandler)

			got := checkTagCooldown(ctx, logger, tt.gitService, "owner", "repo", "v1.0.0", tt.sha, tt.cutoff)

			if got != tt.want {
				t.Errorf("checkTagCooldown() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mockGitService struct {
	getCommitFunc func(ctx context.Context, owner, repo, sha string) (*github.Commit, *github.Response, error)
}

func (m *mockGitService) GetCommit(ctx context.Context, _ *slog.Logger, owner, repo, sha string) (*github.Commit, *github.Response, error) {
	if m.getCommitFunc != nil {
		return m.getCommitFunc(ctx, owner, repo, sha)
	}
	return nil, nil, errors.New("not implemented")
}

func TestController_getLatestVersionFromTags(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name        string
		tags        []*github.RepositoryTag
		listErr     error
		wantVersion string
		wantErr     bool
	}{
		{
			name: "single semver tag",
			tags: []*github.RepositoryTag{
				{Name: new("v1.0.0")},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name: "multiple semver tags - returns highest",
			tags: []*github.RepositoryTag{
				{Name: new("v1.0.0")},
				{Name: new("v2.0.0")},
				{Name: new("v1.5.0")},
			},
			wantVersion: "v2.0.0",
			wantErr:     false,
		},
		{
			name: "mix of valid and invalid semver",
			tags: []*github.RepositoryTag{
				{Name: new("v1.0.0")},
				{Name: new("not-a-version")},
				{Name: new("v2.0.0")},
			},
			wantVersion: "v2.0.0",
			wantErr:     false,
		},
		{
			name: "only invalid versions - returns latest by string comparison",
			tags: []*github.RepositoryTag{
				{Name: new("main")},
				{Name: new("release")},
				{Name: new("develop")},
			},
			wantVersion: "release",
			wantErr:     false,
		},
		{
			name:        "no tags",
			tags:        []*github.RepositoryTag{},
			wantVersion: "",
			wantErr:     false,
		},
		{
			name:        "nil tags",
			tags:        nil,
			wantVersion: "",
			wantErr:     false,
		},
		{
			name: "prerelease versions",
			tags: []*github.RepositoryTag{
				{Name: new("v1.0.0-alpha")},
				{Name: new("v1.0.0-beta")},
				{Name: new("v1.0.0")},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name: "build metadata versions",
			tags: []*github.RepositoryTag{
				{Name: new("v1.0.0+build.1")},
				{Name: new("v1.0.0+build.2")},
				{Name: new("v1.0.1")},
			},
			wantVersion: "v1.0.1",
			wantErr:     false,
		},
		{
			name: "tags with nil names",
			tags: []*github.RepositoryTag{
				{Name: nil},
				{Name: new("v1.0.0")},
				{Name: nil},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name:        "API error",
			tags:        nil,
			listErr:     errors.New("API error"),
			wantVersion: "",
			wantErr:     true,
		},
		{
			name: "empty tag name",
			tags: []*github.RepositoryTag{
				{Name: new("")},
				{Name: new("v1.0.0")},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name: "tags without v prefix",
			tags: []*github.RepositoryTag{
				{Name: new("1.0.0")},
				{Name: new("2.0.0")},
				{Name: new("1.5.0")},
			},
			wantVersion: "2.0.0",
			wantErr:     false,
		},
		{
			name: "mixed v prefix and no prefix",
			tags: []*github.RepositoryTag{
				{Name: new("v1.0.0")},
				{Name: new("2.0.0")},
				{Name: new("v1.5.0")},
			},
			wantVersion: "2.0.0",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockRepo := &mockRepoService{
				listTagsFunc: func(_ context.Context, _, _ string, _ *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error) {
					return tt.tags, nil, tt.listErr
				},
			}

			c := &Controller{
				repositoriesService: newTestRepoService(mockRepo),
			}

			ctx := t.Context()
			logger := slog.New(slog.DiscardHandler)

			gotVersion, err := c.getLatestVersionFromTags(ctx, logger, "owner", "repo", false, time.Time{}, nil)

			if (err != nil) != tt.wantErr {
				t.Errorf("getLatestVersionFromTags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotVersion != tt.wantVersion {
				t.Errorf("getLatestVersionFromTags() = %v, want %v", gotVersion, tt.wantVersion)
			}
		})
	}
}
