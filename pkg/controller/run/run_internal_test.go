package run

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
)

func TestReview_Valid(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name   string
		review *Review
		want   bool
	}{
		{
			name:   "nil review",
			review: nil,
			want:   false,
		},
		{
			name:   "empty review",
			review: &Review{},
			want:   false,
		},
		{
			name: "missing repo owner",
			review: &Review{
				RepoName:    "repo",
				PullRequest: 1,
			},
			want: false,
		},
		{
			name: "missing repo name",
			review: &Review{
				RepoOwner:   "owner",
				PullRequest: 1,
			},
			want: false,
		},
		{
			name: "missing pull request",
			review: &Review{
				RepoOwner: "owner",
				RepoName:  "repo",
			},
			want: false,
		},
		{
			name: "zero pull request",
			review: &Review{
				RepoOwner:   "owner",
				RepoName:    "repo",
				PullRequest: 0,
			},
			want: false,
		},
		{
			name: "valid review",
			review: &Review{
				RepoOwner:   "owner",
				RepoName:    "repo",
				PullRequest: 123,
			},
			want: true,
		},
		{
			name: "valid review with SHA",
			review: &Review{
				RepoOwner:   "owner",
				RepoName:    "repo",
				PullRequest: 456,
				SHA:         "abc123",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.review.Valid(); got != tt.want {
				t.Errorf("Review.Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestController_processLines(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name        string
		lines       []string
		param       *ParamRun
		wantChanged bool
		wantFailed  bool
	}{
		{
			name:        "empty lines",
			lines:       []string{},
			param:       &ParamRun{Stderr: &bytes.Buffer{}},
			wantChanged: false,
			wantFailed:  false,
		},
		{
			name: "no action lines",
			lines: []string{
				"name: Test Workflow",
				"on: push",
				"jobs:",
				"  test:",
				"    runs-on: ubuntu-latest",
			},
			param:       &ParamRun{Stderr: &bytes.Buffer{}},
			wantChanged: false,
			wantFailed:  false,
		},
		{
			name: "already pinned action with semver comment",
			lines: []string{
				"    - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3.5.2",
			},
			param:       &ParamRun{Stderr: &bytes.Buffer{}},
			wantChanged: false,
			wantFailed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			ctrl := New(&github.RepositoriesServiceImpl{
				Tags: map[string]*github.ListTagsResult{
					"actions/checkout/0": {
						Tags: []*github.RepositoryTag{
							{
								Name: new("v3.5.2"),
								Commit: &github.Commit{
									SHA: new("8e5e7e5ab8b370d6c329ec480221332ada57f0ab"),
								},
							},
						},
						Response: &github.Response{},
					},
				},
				Releases: map[string]*github.ListReleasesResult{
					"actions/checkout/0": {
						Releases: []*github.RepositoryRelease{},
						Response: &github.Response{},
					},
				},
				Commits: map[string]*github.GetCommitSHA1Result{},
			}, nil, nil, fs, &config.Config{}, tt.param)

			logger := slog.New(slog.DiscardHandler)
			linesCopy := make([]string, len(tt.lines))
			copy(linesCopy, tt.lines)

			changed, failed := ctrl.processLines(context.Background(), logger, "test.yml", linesCopy)

			if changed != tt.wantChanged {
				t.Errorf("processLines() changed = %v, want %v", changed, tt.wantChanged)
			}
			if failed != tt.wantFailed {
				t.Errorf("processLines() failed = %v, want %v", failed, tt.wantFailed)
			}
		})
	}
}

func TestController_readWorkflow(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name      string
		content   string
		wantLines []string
		wantErr   bool
	}{
		{
			name:      "empty file",
			content:   "",
			wantLines: []string{},
			wantErr:   false,
		},
		{
			name:      "single line",
			content:   "name: Test",
			wantLines: []string{"name: Test"},
			wantErr:   false,
		},
		{
			name: "multiple lines",
			content: `name: Test
on: push
jobs:
  test:
    runs-on: ubuntu-latest`,
			wantLines: []string{
				"name: Test",
				"on: push",
				"jobs:",
				"  test:",
				"    runs-on: ubuntu-latest",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Create a temporary file for testing (readWorkflow uses os.Open, not afero)
			tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			fs := afero.NewMemMapFs()
			ctrl := &Controller{fs: fs}
			lines, err := ctrl.readWorkflow(tmpFile.Name())

			if (err != nil) != tt.wantErr {
				t.Errorf("readWorkflow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(lines) != len(tt.wantLines) {
				t.Errorf("readWorkflow() got %d lines, want %d", len(lines), len(tt.wantLines))
				return
			}

			for i, line := range lines {
				if line != tt.wantLines[i] {
					t.Errorf("readWorkflow() line[%d] = %q, want %q", i, line, tt.wantLines[i])
				}
			}
		})
	}
}

func TestController_readWorkflow_fileNotFound(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	ctrl := &Controller{fs: fs}

	_, err := ctrl.readWorkflow("nonexistent-file-that-does-not-exist.yml")
	if err == nil {
		t.Error("readWorkflow() expected error for non-existent file, got nil")
	}
}

func TestController_writeWorkflow(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		lines       []string
		wantContent string
	}{
		{
			name:        "empty lines",
			lines:       []string{},
			wantContent: "\n",
		},
		{
			name:        "single line",
			lines:       []string{"name: Test"},
			wantContent: "name: Test\n",
		},
		{
			name: "multiple lines",
			lines: []string{
				"name: Test",
				"on: push",
				"jobs:",
			},
			wantContent: "name: Test\non: push\njobs:\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			ctrl := &Controller{fs: fs}

			err := ctrl.writeWorkflow("test.yml", tt.lines)
			if err != nil {
				t.Errorf("writeWorkflow() error = %v", err)
				return
			}

			content, err := afero.ReadFile(fs, "test.yml")
			if err != nil {
				t.Errorf("failed to read written file: %v", err)
				return
			}

			if string(content) != tt.wantContent {
				t.Errorf("writeWorkflow() wrote %q, want %q", string(content), tt.wantContent)
			}
		})
	}
}
