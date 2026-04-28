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
		name                string
		content             string
		wantLines           []string
		wantLineEnding      string
		wantTrailingNewline bool
		wantErr             bool
	}{
		{
			name:                "empty file",
			content:             "",
			wantLines:           []string{},
			wantLineEnding:      "\n",
			wantTrailingNewline: false,
		},
		{
			name:                "single line no trailing newline",
			content:             "name: Test",
			wantLines:           []string{"name: Test"},
			wantLineEnding:      "\n",
			wantTrailingNewline: false,
		},
		{
			name:                "LF multiple lines with trailing newline",
			content:             "name: Test\non: push\njobs:\n",
			wantLines:           []string{"name: Test", "on: push", "jobs:"},
			wantLineEnding:      "\n",
			wantTrailingNewline: true,
		},
		{
			name:                "LF multiple lines without trailing newline",
			content:             "name: Test\non: push\njobs:",
			wantLines:           []string{"name: Test", "on: push", "jobs:"},
			wantLineEnding:      "\n",
			wantTrailingNewline: false,
		},
		{
			name:                "CRLF multiple lines with trailing newline",
			content:             "name: Test\r\non: push\r\njobs:\r\n",
			wantLines:           []string{"name: Test", "on: push", "jobs:"},
			wantLineEnding:      "\r\n",
			wantTrailingNewline: true,
		},
		{
			name:                "CRLF multiple lines without trailing newline",
			content:             "name: Test\r\non: push\r\njobs:",
			wantLines:           []string{"name: Test", "on: push", "jobs:"},
			wantLineEnding:      "\r\n",
			wantTrailingNewline: false,
		},
		{
			name:                "only LF",
			content:             "\n",
			wantLines:           []string{},
			wantLineEnding:      "\n",
			wantTrailingNewline: true,
		},
		{
			name:                "only CRLF",
			content:             "\r\n",
			wantLines:           []string{},
			wantLineEnding:      "\r\n",
			wantTrailingNewline: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Create a temporary file for testing (readWorkflow uses os.ReadFile, not afero)
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
			lines, format, err := ctrl.readWorkflow(tmpFile.Name())

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

			if format.lineEnding != tt.wantLineEnding {
				t.Errorf("readWorkflow() lineEnding = %q, want %q", format.lineEnding, tt.wantLineEnding)
			}
			if format.trailingNewline != tt.wantTrailingNewline {
				t.Errorf("readWorkflow() trailingNewline = %v, want %v", format.trailingNewline, tt.wantTrailingNewline)
			}
		})
	}
}

func TestController_readWorkflow_fileNotFound(t *testing.T) {
	t.Parallel()
	fs := afero.NewMemMapFs()
	ctrl := &Controller{fs: fs}

	_, _, err := ctrl.readWorkflow("nonexistent-file-that-does-not-exist.yml")
	if err == nil {
		t.Error("readWorkflow() expected error for non-existent file, got nil")
	}
}

func TestController_writeWorkflow(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name        string
		lines       []string
		format      *fileFormat
		wantContent string
	}{
		{
			name:        "empty lines no trailing newline",
			lines:       []string{},
			format:      &fileFormat{lineEnding: "\n", trailingNewline: false},
			wantContent: "",
		},
		{
			name:        "empty lines with trailing newline",
			lines:       []string{},
			format:      &fileFormat{lineEnding: "\n", trailingNewline: true},
			wantContent: "\n",
		},
		{
			name:        "single line LF",
			lines:       []string{"name: Test"},
			format:      &fileFormat{lineEnding: "\n", trailingNewline: true},
			wantContent: "name: Test\n",
		},
		{
			name:        "single line LF no trailing",
			lines:       []string{"name: Test"},
			format:      &fileFormat{lineEnding: "\n", trailingNewline: false},
			wantContent: "name: Test",
		},
		{
			name:        "multiple lines LF",
			lines:       []string{"name: Test", "on: push", "jobs:"},
			format:      &fileFormat{lineEnding: "\n", trailingNewline: true},
			wantContent: "name: Test\non: push\njobs:\n",
		},
		{
			name:        "multiple lines CRLF",
			lines:       []string{"name: Test", "on: push", "jobs:"},
			format:      &fileFormat{lineEnding: "\r\n", trailingNewline: true},
			wantContent: "name: Test\r\non: push\r\njobs:\r\n",
		},
		{
			name:        "multiple lines CRLF no trailing",
			lines:       []string{"name: Test", "on: push", "jobs:"},
			format:      &fileFormat{lineEnding: "\r\n", trailingNewline: false},
			wantContent: "name: Test\r\non: push\r\njobs:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			ctrl := &Controller{fs: fs}

			err := ctrl.writeWorkflow("test.yml", tt.lines, tt.format)
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

// TestController_readWriteWorkflow_roundTrip verifies that the read → write
// cycle preserves the file's original line endings and trailing-newline state
// (issue #1492).
func TestController_readWriteWorkflow_roundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
	}{
		{name: "LF with trailing", content: "a\nb\nc\n"},
		{name: "LF without trailing", content: "a\nb\nc"},
		{name: "CRLF with trailing", content: "a\r\nb\r\nc\r\n"},
		{name: "CRLF without trailing", content: "a\r\nb\r\nc"},
		{name: "single line no trailing", content: "only"},
		{name: "empty file", content: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpFile, err := os.CreateTemp(t.TempDir(), "test-*.yml")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Read with the real filesystem; write into an afero MemMapFs at the
			// same path and compare bytes.
			fs := afero.NewMemMapFs()
			ctrl := &Controller{fs: fs}

			lines, format, err := ctrl.readWorkflow(tmpFile.Name())
			if err != nil {
				t.Fatalf("readWorkflow() error = %v", err)
			}
			if err := ctrl.writeWorkflow(tmpFile.Name(), lines, format); err != nil {
				t.Fatalf("writeWorkflow() error = %v", err)
			}

			got, err := afero.ReadFile(fs, tmpFile.Name())
			if err != nil {
				t.Fatalf("failed to read back: %v", err)
			}
			if string(got) != tt.content {
				t.Errorf("round trip changed content: got %q, want %q", string(got), tt.content)
			}
		})
	}
}
