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

// mockRepositoriesService is a mock implementation of RepositoriesService for testing
type mockRepositoriesService struct {
	listReleasesFunc func(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error)
	listTagsFunc     func(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error)
}

func (m *mockRepositoriesService) ListTags(ctx context.Context, owner string, repo string, opts *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error) {
	if m.listTagsFunc != nil {
		return m.listTagsFunc(ctx, owner, repo, opts)
	}
	return nil, nil, errors.New("not implemented")
}

func (m *mockRepositoriesService) GetCommitSHA1(_ context.Context, _, _, _, _ string) (string, *github.Response, error) {
	return "", nil, errors.New("not implemented")
}

func (m *mockRepositoriesService) ListReleases(ctx context.Context, owner, repo string, opts *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error) {
	if m.listReleasesFunc != nil {
		return m.listReleasesFunc(ctx, owner, repo, opts)
	}
	return nil, nil, errors.New("not implemented")
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
				{TagName: github.Ptr("v1.0.0")},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name: "multiple semver releases - returns highest",
			releases: []*github.RepositoryRelease{
				{TagName: github.Ptr("v1.0.0")},
				{TagName: github.Ptr("v2.0.0")},
				{TagName: github.Ptr("v1.5.0")},
			},
			wantVersion: "v2.0.0",
			wantErr:     false,
		},
		{
			name: "mix of valid and invalid semver",
			releases: []*github.RepositoryRelease{
				{TagName: github.Ptr("v1.0.0")},
				{TagName: github.Ptr("not-a-version")},
				{TagName: github.Ptr("v2.0.0")},
			},
			wantVersion: "v2.0.0",
			wantErr:     false,
		},
		{
			name: "only invalid versions - returns latest by string comparison",
			releases: []*github.RepositoryRelease{
				{TagName: github.Ptr("main")},
				{TagName: github.Ptr("release")},
				{TagName: github.Ptr("develop")},
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
				{TagName: github.Ptr("v1.0.0-alpha")},
				{TagName: github.Ptr("v1.0.0-beta")},
				{TagName: github.Ptr("v1.0.0")},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name: "build metadata versions",
			releases: []*github.RepositoryRelease{
				{TagName: github.Ptr("v1.0.0+build.1")},
				{TagName: github.Ptr("v1.0.0+build.2")},
				{TagName: github.Ptr("v1.0.1")},
			},
			wantVersion: "v1.0.1",
			wantErr:     false,
		},
		{
			name: "releases with nil tag names",
			releases: []*github.RepositoryRelease{
				{TagName: nil},
				{TagName: github.Ptr("v1.0.0")},
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
				{TagName: github.Ptr("")},
				{TagName: github.Ptr("v1.0.0")},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name: "stable version ignores prerelease when current is stable (issue #1095)",
			releases: []*github.RepositoryRelease{
				{TagName: github.Ptr("v6-beta"), Prerelease: github.Ptr(true)},
				{TagName: github.Ptr("v5.0.0"), Prerelease: github.Ptr(false)},
				{TagName: github.Ptr("v4.3.0"), Prerelease: github.Ptr(false)},
			},
			isStable:    true,
			wantVersion: "v5.0.0",
			wantErr:     false,
		},
		{
			name: "prerelease version can update to newer prerelease (issue #1095)",
			releases: []*github.RepositoryRelease{
				{TagName: github.Ptr("v6-beta"), Prerelease: github.Ptr(true)},
				{TagName: github.Ptr("v5.0.0"), Prerelease: github.Ptr(false)},
			},
			isStable:    false,
			wantVersion: "v6-beta",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockRepo := &mockRepositoriesService{
				listReleasesFunc: func(_ context.Context, _, _ string, _ *github.ListOptions) ([]*github.RepositoryRelease, *github.Response, error) {
					return tt.releases, nil, tt.listErr
				},
			}

			c := &Controller{
				repositoriesService: mockRepo,
			}

			ctx := t.Context()
			logger := slog.New(slog.DiscardHandler)

			gotVersion, err := c.getLatestVersionFromReleases(ctx, logger, "owner", "repo", tt.isStable, time.Time{})

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
				{Name: github.Ptr("v1.0.0")},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name: "multiple semver tags - returns highest",
			tags: []*github.RepositoryTag{
				{Name: github.Ptr("v1.0.0")},
				{Name: github.Ptr("v2.0.0")},
				{Name: github.Ptr("v1.5.0")},
			},
			wantVersion: "v2.0.0",
			wantErr:     false,
		},
		{
			name: "mix of valid and invalid semver",
			tags: []*github.RepositoryTag{
				{Name: github.Ptr("v1.0.0")},
				{Name: github.Ptr("not-a-version")},
				{Name: github.Ptr("v2.0.0")},
			},
			wantVersion: "v2.0.0",
			wantErr:     false,
		},
		{
			name: "only invalid versions - returns latest by string comparison",
			tags: []*github.RepositoryTag{
				{Name: github.Ptr("main")},
				{Name: github.Ptr("release")},
				{Name: github.Ptr("develop")},
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
				{Name: github.Ptr("v1.0.0-alpha")},
				{Name: github.Ptr("v1.0.0-beta")},
				{Name: github.Ptr("v1.0.0")},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name: "build metadata versions",
			tags: []*github.RepositoryTag{
				{Name: github.Ptr("v1.0.0+build.1")},
				{Name: github.Ptr("v1.0.0+build.2")},
				{Name: github.Ptr("v1.0.1")},
			},
			wantVersion: "v1.0.1",
			wantErr:     false,
		},
		{
			name: "tags with nil names",
			tags: []*github.RepositoryTag{
				{Name: nil},
				{Name: github.Ptr("v1.0.0")},
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
				{Name: github.Ptr("")},
				{Name: github.Ptr("v1.0.0")},
			},
			wantVersion: "v1.0.0",
			wantErr:     false,
		},
		{
			name: "tags without v prefix",
			tags: []*github.RepositoryTag{
				{Name: github.Ptr("1.0.0")},
				{Name: github.Ptr("2.0.0")},
				{Name: github.Ptr("1.5.0")},
			},
			wantVersion: "2.0.0",
			wantErr:     false,
		},
		{
			name: "mixed v prefix and no prefix",
			tags: []*github.RepositoryTag{
				{Name: github.Ptr("v1.0.0")},
				{Name: github.Ptr("2.0.0")},
				{Name: github.Ptr("v1.5.0")},
			},
			wantVersion: "2.0.0",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockRepo := &mockRepositoriesService{
				listTagsFunc: func(_ context.Context, _, _ string, _ *github.ListOptions) ([]*github.RepositoryTag, *github.Response, error) {
					return tt.tags, nil, tt.listErr
				},
			}

			c := &Controller{
				repositoriesService: mockRepo,
			}

			ctx := t.Context()
			logger := slog.New(slog.DiscardHandler)

			gotVersion, err := c.getLatestVersionFromTags(ctx, logger, "owner", "repo", false, time.Time{})

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
