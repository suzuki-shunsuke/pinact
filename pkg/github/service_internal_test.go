package github

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"
)

// TestRepositoriesServiceImpl_ResolveTagSHA covers tag-only resolution with the
// opt-in branch fallback. By default, refs that don't resolve to a tag error
// with errNotATag. When allowBranch is true, resolution falls back to the bare
// ref (preserving the pre-refusal behavior) and a warning is logged.
func TestRepositoriesServiceImpl_ResolveTagSHA(t *testing.T) { //nolint:funlen
	t.Parallel()

	ctx := context.Background()
	logger := slog.New(slog.DiscardHandler)

	notFound := &Response{Response: &http.Response{StatusCode: http.StatusNotFound}}
	serverErr := &Response{Response: &http.Response{StatusCode: http.StatusInternalServerError}}

	tests := []struct {
		name          string
		owner         string
		repo          string
		ref           string
		allowBranch   bool
		cache         map[string]*GetCommitSHA1Result
		wantSHA       string
		wantErr       bool
		wantIsNotATag bool
	}{
		{
			name:  "tag only - resolves via tag namespace",
			owner: "example-owner",
			repo:  "tag-only-action",
			ref:   "v3",
			cache: map[string]*GetCommitSHA1Result{
				"example-owner/tag-only-action/tags/v3": {SHA: "tagSHA"},
			},
			wantSHA: "tagSHA",
		},
		{
			name:  "branch only, allow_branch_pins disabled - errors with errNotATag",
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
			wantErr:       true,
			wantIsNotATag: true,
		},
		{
			name:        "branch only, allow_branch_pins enabled - falls back to branch",
			owner:       "example-owner",
			repo:        "branch-only-action",
			ref:         "v1",
			allowBranch: true,
			cache: map[string]*GetCommitSHA1Result{
				"example-owner/branch-only-action/tags/v1": {
					Response: notFound,
					err:      errors.New("404 Not Found"),
				},
				"example-owner/branch-only-action/v1": {SHA: "branchSHA"},
			},
			wantSHA: "branchSHA",
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
			wantSHA: "tagSHADiverged",
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
			wantSHA: "tagSHAAncestor",
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
			name:        "both lookups 404 with allow_branch_pins - propagates error from fallback",
			owner:       "example-owner",
			repo:        "missing-action",
			ref:         "v9",
			allowBranch: true,
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

			sha, err := r.ResolveTagSHA(ctx, logger, tt.owner, tt.repo, tt.ref, tt.allowBranch)

			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveTagSHA() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantIsNotATag && !errors.Is(err, errNotATag) {
				t.Errorf("ResolveTagSHA() error = %v, want errors.Is(err, errNotATag)", err)
			}
			if tt.wantErr {
				return
			}
			if sha != tt.wantSHA {
				t.Errorf("sha = %q, want %q", sha, tt.wantSHA)
			}
		})
	}
}
