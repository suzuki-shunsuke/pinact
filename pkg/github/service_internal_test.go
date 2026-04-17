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
			owner: "example-owner",
			repo:  "tag-only-action",
			ref:   "v3",
			cache: map[string]*GetCommitSHA1Result{
				"example-owner/tag-only-action/tags/v3": {SHA: "tagSHA"},
			},
			wantSHA:     "tagSHA",
			wantFromTag: true,
		},
		{
			name:  "branch only - falls back after 404 on tag namespace",
			owner: "example-owner",
			repo:  "branch-only-action",
			ref:   "v1",
			cache: map[string]*GetCommitSHA1Result{
				"example-owner/branch-only-action/tags/v1": {
					Response: notFound,
					err:      errors.New("404 Not Found"),
				},
				"example-owner/branch-only-action/v1": {SHA: "branchSHA"},
			},
			wantSHA:     "branchSHA",
			wantFromTag: false,
		},
		{
			name:  "diverged tag and branch - tag wins",
			owner: "example-owner",
			repo:  "diverged-action",
			ref:   "v5",
			cache: map[string]*GetCommitSHA1Result{
				"example-owner/diverged-action/v5":      {SHA: "branchSHADiverged"},
				"example-owner/diverged-action/tags/v5": {SHA: "tagSHADiverged"},
			},
			wantSHA:     "tagSHADiverged",
			wantFromTag: true,
		},
		{
			name:  "branch is ancestor of tag - tag still wins",
			owner: "example-owner",
			repo:  "ancestor-action",
			ref:   "v3",
			cache: map[string]*GetCommitSHA1Result{
				"example-owner/ancestor-action/v3":      {SHA: "branchSHAAncestor"},
				"example-owner/ancestor-action/tags/v3": {SHA: "tagSHAAncestor"},
			},
			wantSHA:     "tagSHAAncestor",
			wantFromTag: true,
		},
		{
			name:  "non-404 error on tag lookup - propagates without fallback",
			owner: "example-owner",
			repo:  "server-error-action",
			ref:   "v1",
			cache: map[string]*GetCommitSHA1Result{
				"example-owner/server-error-action/tags/v1": {
					Response: serverErr,
					err:      errors.New("500 Internal Server Error"),
				},
				"example-owner/server-error-action/v1": {SHA: "branchSHA"},
			},
			wantErr: true,
		},
		{
			name:  "both lookups 404 - propagates error from fallback",
			owner: "example-owner",
			repo:  "missing-action",
			ref:   "v9",
			cache: map[string]*GetCommitSHA1Result{
				"example-owner/missing-action/tags/v9": {Response: notFound, err: errors.New("404 Not Found")},
				"example-owner/missing-action/v9":      {Response: notFound, err: errors.New("404 Not Found")},
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
