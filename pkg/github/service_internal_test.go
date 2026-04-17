package github

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"
)

// TestRepositoriesServiceImpl_ResolveCommitSHAPreferTag covers the branch-vs-tag
// precedence fix: when a ref exists as both a tag and a branch, pinact must
// resolve to the tag to match what GitHub Actions executes at runtime.
func TestRepositoriesServiceImpl_ResolveCommitSHAPreferTag(t *testing.T) { //nolint:funlen
	t.Parallel()

	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	notFound := &Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
	serverErr := &Response{Response: &http.Response{StatusCode: http.StatusInternalServerError}}

	tests := []struct {
		name        string
		owner       string
		repo        string
		ref         string
		cache       map[string]*GetCommitSHA1Result
		wantSHA     string
		wantFromTag bool
		wantErr     bool
	}{
		{
			name:  "tag only - resolves via tag namespace",
			owner: "actions",
			repo:  "checkout",
			ref:   "v3",
			cache: map[string]*GetCommitSHA1Result{
				"actions/checkout/tags/v3": {SHA: "tagSHA"},
			},
			wantSHA:     "tagSHA",
			wantFromTag: true,
		},
		{
			name:  "branch only - falls back after 404 on tag namespace",
			owner: "allenheltondev",
			repo:  "detect-breaking-changes-action",
			ref:   "v1",
			cache: map[string]*GetCommitSHA1Result{
				"allenheltondev/detect-breaking-changes-action/tags/v1": {
					Response: notFound,
					err:      errors.New("404 Not Found"),
				},
				"allenheltondev/detect-breaking-changes-action/v1": {SHA: "branchSHA"},
			},
			wantSHA:     "branchSHA",
			wantFromTag: false,
		},
		{
			name:  "diverged tag and branch - tag wins",
			owner: "peter-evans",
			repo:  "slash-command-dispatch",
			ref:   "v5",
			cache: map[string]*GetCommitSHA1Result{
				"peter-evans/slash-command-dispatch/v5":      {SHA: "0683e68c"},
				"peter-evans/slash-command-dispatch/tags/v5": {SHA: "9bdcd791"},
			},
			wantSHA:     "9bdcd791",
			wantFromTag: true,
		},
		{
			name:  "branch is ancestor of tag - tag still wins",
			owner: "thollander",
			repo:  "actions-comment-pull-request",
			ref:   "v3",
			cache: map[string]*GetCommitSHA1Result{
				"thollander/actions-comment-pull-request/v3":      {SHA: "65f9e5c9"},
				"thollander/actions-comment-pull-request/tags/v3": {SHA: "24bffb9b"},
			},
			wantSHA:     "24bffb9b",
			wantFromTag: true,
		},
		{
			name:  "non-404 error on tag lookup - propagates without fallback",
			owner: "owner",
			repo:  "repo",
			ref:   "v1",
			cache: map[string]*GetCommitSHA1Result{
				"owner/repo/tags/v1": {
					Response: serverErr,
					err:      errors.New("500 Internal Server Error"),
				},
				"owner/repo/v1": {SHA: "branchSHA"},
			},
			wantErr: true,
		},
		{
			name:  "both lookups 404 - propagates error from fallback",
			owner: "nobody",
			repo:  "nothing",
			ref:   "v9",
			cache: map[string]*GetCommitSHA1Result{
				"nobody/nothing/tags/v9": {Response: notFound, err: errors.New("404 Not Found")},
				"nobody/nothing/v9":      {Response: notFound, err: errors.New("404 Not Found")},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &RepositoriesServiceImpl{Commits: tt.cache}

			sha, fromTag, err := r.ResolveCommitSHAPreferTag(ctx, logger, tt.owner, tt.repo, tt.ref)

			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveCommitSHAPreferTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if sha != tt.wantSHA {
				t.Errorf("sha = %q, want %q", sha, tt.wantSHA)
			}
			if fromTag != tt.wantFromTag {
				t.Errorf("fromTag = %v, want %v", fromTag, tt.wantFromTag)
			}
		})
	}
}
