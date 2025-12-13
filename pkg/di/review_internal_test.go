package di

import (
	"testing"

	"github.com/lmittmann/tint"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/flag"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/run"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
)

func newTestLogger() *slogutil.Logger {
	return slogutil.New(&slogutil.InputNew{
		Name:        "test",
		Version:     "test",
		TintOptions: &tint.Options{NoColor: true},
	})
}

func Test_populateReviewFromGitHubActionsEnv(t *testing.T) {
	t.Parallel()
	t.Run("already has repo name", func(t *testing.T) {
		t.Parallel()
		review := &run.Review{RepoName: "existing-repo"}
		flags := &Flags{GitHubRepository: "owner/other-repo"}
		if err := populateReviewFromGitHubActionsEnv(afero.NewMemMapFs(), review, flags); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if review.RepoName != "existing-repo" {
			t.Errorf("RepoName: wanted %q, got %q", "existing-repo", review.RepoName)
		}
	})

	t.Run("extract repo name from GITHUB_REPOSITORY", func(t *testing.T) {
		t.Parallel()
		review := &run.Review{}
		flags := &Flags{GitHubRepository: "owner/my-repo"}
		if err := populateReviewFromGitHubActionsEnv(afero.NewMemMapFs(), review, flags); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if review.RepoName != "my-repo" {
			t.Errorf("RepoName: wanted %q, got %q", "my-repo", review.RepoName)
		}
	})

	t.Run("invalid GITHUB_REPOSITORY - no slash", func(t *testing.T) {
		t.Parallel()
		review := &run.Review{}
		flags := &Flags{GitHubRepository: "noslash"}
		if err := populateReviewFromGitHubActionsEnv(afero.NewMemMapFs(), review, flags); err == nil {
			t.Error("expected error, got nil")
		}
	})

	t.Run("extract PR number from event", func(t *testing.T) {
		t.Parallel()
		fs := afero.NewMemMapFs()
		eventPath := "/tmp/event.json"
		eventContent := `{"pull_request": {"number": 42, "head": {"sha": "abc123"}}}`
		if err := afero.WriteFile(fs, eventPath, []byte(eventContent), 0o644); err != nil {
			t.Fatal(err)
		}
		review := &run.Review{RepoName: "my-repo"}
		flags := &Flags{GitHubEventPath: eventPath}
		if err := populateReviewFromGitHubActionsEnv(fs, review, flags); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if review.PullRequest != 42 {
			t.Errorf("PullRequest: wanted %d, got %d", 42, review.PullRequest)
		}
		if review.SHA != "abc123" {
			t.Errorf("SHA: wanted %q, got %q", "abc123", review.SHA)
		}
	})
}

func Test_setupReview(t *testing.T) {
	t.Parallel()
	logger := newTestLogger()

	t.Run("review disabled", func(t *testing.T) {
		t.Parallel()
		flags := &Flags{GlobalFlags: &flag.GlobalFlags{}, Review: false}
		if got := setupReview(afero.NewMemMapFs(), logger, flags); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("review enabled with all fields", func(t *testing.T) {
		t.Parallel()
		flags := &Flags{
			GlobalFlags: &flag.GlobalFlags{},
			Review:      true,
			RepoOwner:   "owner",
			RepoName:    "repo",
			PR:          42,
			SHA:         "abc123",
		}
		got := setupReview(afero.NewMemMapFs(), logger, flags)
		if got == nil {
			t.Fatal("expected non-nil, got nil")
		}
		if got.RepoOwner != "owner" {
			t.Errorf("RepoOwner: wanted %q, got %q", "owner", got.RepoOwner)
		}
		if got.RepoName != "repo" {
			t.Errorf("RepoName: wanted %q, got %q", "repo", got.RepoName)
		}
		if got.PullRequest != 42 {
			t.Errorf("PullRequest: wanted %d, got %d", 42, got.PullRequest)
		}
	})

	t.Run("review enabled but invalid - missing required fields", func(t *testing.T) {
		t.Parallel()
		flags := &Flags{GlobalFlags: &flag.GlobalFlags{}, Review: true}
		if got := setupReview(afero.NewMemMapFs(), logger, flags); got != nil {
			t.Errorf("expected nil for invalid review, got %+v", got)
		}
	})
}
