package github

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/go-github/v83/github"
)

// RepositoriesService defines the interface for GitHub Repositories API operations.
type RepositoriesService interface {
	ListTags(ctx context.Context, owner string, repo string, opts *ListOptions) ([]*RepositoryTag, *Response, error)
	GetCommitSHA1(ctx context.Context, owner, repo, ref, lastSHA string) (string, *Response, error)
	ListReleases(ctx context.Context, owner, repo string, opts *ListOptions) ([]*RepositoryRelease, *Response, error)
	Get(ctx context.Context, owner, repo string) (*Repository, *Response, error)
}

// PullRequestsService defines the interface for GitHub Pull Requests API operations.
type PullRequestsService interface {
	CreateComment(ctx context.Context, owner, repo string, number int, comment *PullRequestComment) (*PullRequestComment, *Response, error)
}

// GitService defines the interface for GitHub Git API operations.
type GitService interface {
	GetCommit(ctx context.Context, owner, repo, sha string) (*Commit, *Response, error)
}

// repoHost represents which GitHub host a repository belongs to.
type repoHost int

const (
	repoHostUnknown repoHost = iota
	repoHostGHES
	repoHostGitHubDotCom
)

// ClientResolver resolves which GitHub service (GHES or github.com) to use for a given repository.
// It uses the Get a Repository API to check if a repository exists on GHES or github.com,
// and caches the result to avoid redundant API calls.
type ClientResolver struct {
	defaultRepoService RepositoriesService
	defaultGitService  GitService
	ghesRepoService    RepositoriesService
	ghesGitService     GitService
	// repoHosts caches which host a repository belongs to
	repoHosts map[string]repoHost
	// fallback controls whether to fallback to github.com when a repository is not found on GHES
	fallback bool
}

// NewClientResolver creates a new ClientResolver with the given services.
func NewClientResolver(
	defaultRepoService RepositoriesService,
	defaultGitService GitService,
	ghesRepoService RepositoriesService,
	ghesGitService GitService,
	fallback bool,
) *ClientResolver {
	return &ClientResolver{
		defaultRepoService: defaultRepoService,
		defaultGitService:  defaultGitService,
		ghesRepoService:    ghesRepoService,
		ghesGitService:     ghesGitService,
		fallback:           fallback,
		repoHosts:          map[string]repoHost{},
	}
}

// GetRepositoriesService returns the appropriate RepositoriesService for the given repository.
func (r *ClientResolver) GetRepositoriesService(ctx context.Context, logger *slog.Logger, owner, repo string) (RepositoriesService, error) {
	host, err := r.resolveRepoHost(ctx, logger, owner, repo)
	if err != nil {
		return nil, err
	}
	if host == repoHostGitHubDotCom {
		return r.defaultRepoService, nil
	}
	return r.ghesRepoService, nil
}

// GetGitService returns the appropriate GitService for the given repository.
func (r *ClientResolver) GetGitService(ctx context.Context, logger *slog.Logger, owner, repo string) (GitService, error) {
	host, err := r.resolveRepoHost(ctx, logger, owner, repo)
	if err != nil {
		return nil, err
	}
	if host == repoHostGitHubDotCom {
		return r.defaultGitService, nil
	}
	return r.ghesGitService, nil
}

// resolveRepoHost determines which host a repository belongs to using the Get a Repository API.
// If fallback is disabled, it always uses GHES without checking repository existence.
// If fallback is enabled, it checks GHES first and falls back to github.com if not found.
func (r *ClientResolver) resolveRepoHost(ctx context.Context, logger *slog.Logger, owner, repo string) (repoHost, error) {
	// If GHES is not configured, use github.com
	if r.ghesRepoService == nil {
		return repoHostGitHubDotCom, nil
	}

	// If fallback is disabled, always use GHES without checking
	if !r.fallback {
		return repoHostGHES, nil
	}

	key := owner + "/" + repo

	// Check cache first
	if host, ok := r.repoHosts[key]; ok {
		return host, nil
	}

	// Fallback is enabled: check if repository exists on GHES
	_, resp, err := r.ghesRepoService.Get(ctx, owner, repo)
	if err == nil {
		logger.Debug("repository found on GHES", "owner", owner, "repo", repo)
		r.repoHosts[key] = repoHostGHES
		return repoHostGHES, nil
	}

	// If GHES returned 404, check github.com
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		_, resp, err = r.defaultRepoService.Get(ctx, owner, repo)
		if err == nil {
			logger.Debug("repository found on github.com (fallback)", "owner", owner, "repo", repo)
			r.repoHosts[key] = repoHostGitHubDotCom
			return repoHostGitHubDotCom, nil
		}
		// Repository not found on either host
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return repoHostUnknown, fmt.Errorf("repository %s/%s not found on GHES or github.com", owner, repo)
		}
	}

	// Other error from GHES or github.com
	return repoHostUnknown, fmt.Errorf("failed to check repository %s/%s: %w", owner, repo, err)
}

// GitServiceImpl wraps a GitService with caching and GHES fallback support.
type GitServiceImpl struct {
	resolver *ClientResolver
	Commits  map[string]*GetCommitResult
}

// SetResolver sets the ClientResolver for the GitServiceImpl.
func (g *GitServiceImpl) SetResolver(resolver *ClientResolver) {
	g.resolver = resolver
}

// GetCommitResult holds the cached result of a GetCommit call.
type GetCommitResult struct {
	Commit   *Commit
	Response *Response
	err      error
}

// GetCommit retrieves a commit object with caching and GHES fallback.
func (g *GitServiceImpl) GetCommit(ctx context.Context, logger *slog.Logger, owner, repo, sha string) (*Commit, *Response, error) {
	key := fmt.Sprintf("%s/%s/%s", owner, repo, sha)
	if result, ok := g.Commits[key]; ok {
		return result.Commit, result.Response, result.err
	}

	commit, resp, err := g.getCommit(ctx, logger, owner, repo, sha)
	g.Commits[key] = &GetCommitResult{
		Commit:   commit,
		Response: resp,
		err:      err,
	}
	return commit, resp, err
}

// getCommit calls the appropriate GitService based on the repository host.
func (g *GitServiceImpl) getCommit(ctx context.Context, logger *slog.Logger, owner, repo, sha string) (*Commit, *Response, error) {
	service, err := g.resolver.GetGitService(ctx, logger, owner, repo)
	if err != nil {
		return nil, nil, err
	}
	return service.GetCommit(ctx, owner, repo, sha) //nolint:wrapcheck
}

// ListTagsResult holds the cached result of a ListTags call.
type ListTagsResult struct {
	Tags     []*RepositoryTag
	Response *Response
	err      error
}

// ListReleasesResult holds the cached result of a ListReleases call.
type ListReleasesResult struct {
	Releases []*RepositoryRelease
	Response *Response
	err      error
}

// RepositoriesServiceImpl wraps a RepositoriesService with caching and GHES fallback support.
type RepositoriesServiceImpl struct {
	resolver *ClientResolver
	Tags     map[string]*ListTagsResult
	Commits  map[string]*GetCommitSHA1Result
	Releases map[string]*ListReleasesResult
}

// SetResolver sets the ClientResolver for the RepositoriesServiceImpl.
func (r *RepositoriesServiceImpl) SetResolver(resolver *ClientResolver) {
	r.resolver = resolver
}

// Get fetches a repository to check its existence.
func (r *RepositoriesServiceImpl) Get(ctx context.Context, logger *slog.Logger, owner, repo string) (*Repository, *Response, error) {
	service, err := r.resolver.GetRepositoriesService(ctx, logger, owner, repo)
	if err != nil {
		return nil, nil, err
	}
	return service.Get(ctx, owner, repo) //nolint:wrapcheck
}

// GetCommitSHA1 retrieves the commit SHA for a given reference with caching and GHES fallback.
func (r *RepositoriesServiceImpl) GetCommitSHA1(ctx context.Context, logger *slog.Logger, owner, repo, ref, lastSHA string) (string, *Response, error) {
	key := fmt.Sprintf("%s/%s/%s", owner, repo, ref)
	if result, ok := r.Commits[key]; ok {
		return result.SHA, result.Response, result.err
	}

	sha, resp, err := r.getCommitSHA1(ctx, logger, owner, repo, ref, lastSHA)
	r.Commits[key] = &GetCommitSHA1Result{
		SHA:      sha,
		Response: resp,
		err:      err,
	}
	return sha, resp, err
}

// GetCommitSHA1Result holds the cached result of a GetCommitSHA1 call.
type GetCommitSHA1Result struct {
	SHA      string
	Response *Response
	err      error
}

// ListTags retrieves repository tags with caching and GHES fallback.
func (r *RepositoriesServiceImpl) ListTags(ctx context.Context, logger *slog.Logger, owner string, repo string, opts *ListOptions) ([]*RepositoryTag, *Response, error) {
	key := fmt.Sprintf("%s/%s/%v", owner, repo, opts.Page)
	if result, ok := r.Tags[key]; ok {
		return result.Tags, result.Response, result.err
	}

	tags, resp, err := r.listTags(ctx, logger, owner, repo, opts)
	r.Tags[key] = &ListTagsResult{
		Tags:     tags,
		Response: resp,
		err:      err,
	}
	return tags, resp, err
}

// ListReleases retrieves repository releases with caching and GHES fallback.
func (r *RepositoriesServiceImpl) ListReleases(ctx context.Context, logger *slog.Logger, owner string, repo string, opts *ListOptions) ([]*RepositoryRelease, *Response, error) {
	key := fmt.Sprintf("%s/%s/%v", owner, repo, opts.Page)
	if result, ok := r.Releases[key]; ok {
		return result.Releases, result.Response, result.err
	}

	releases, resp, err := r.listReleases(ctx, logger, owner, repo, opts)
	arr := filterDraftReleases(releases)
	r.Releases[key] = &ListReleasesResult{
		Releases: arr,
		Response: resp,
		err:      err,
	}
	return arr, resp, err
}

// getCommitSHA1 calls the appropriate RepositoriesService based on the repository host.
func (r *RepositoriesServiceImpl) getCommitSHA1(ctx context.Context, logger *slog.Logger, owner, repo, ref, lastSHA string) (string, *Response, error) {
	service, err := r.resolver.GetRepositoriesService(ctx, logger, owner, repo)
	if err != nil {
		return "", nil, err
	}
	return service.GetCommitSHA1(ctx, owner, repo, ref, lastSHA) //nolint:wrapcheck
}

// listTags calls the appropriate RepositoriesService based on the repository host.
func (r *RepositoriesServiceImpl) listTags(ctx context.Context, logger *slog.Logger, owner string, repo string, opts *ListOptions) ([]*RepositoryTag, *Response, error) {
	service, err := r.resolver.GetRepositoriesService(ctx, logger, owner, repo)
	if err != nil {
		return nil, nil, err
	}
	return service.ListTags(ctx, owner, repo, opts) //nolint:wrapcheck
}

// listReleases calls the appropriate RepositoriesService based on the repository host.
func (r *RepositoriesServiceImpl) listReleases(ctx context.Context, logger *slog.Logger, owner string, repo string, opts *ListOptions) ([]*RepositoryRelease, *Response, error) {
	service, err := r.resolver.GetRepositoriesService(ctx, logger, owner, repo)
	if err != nil {
		return nil, nil, err
	}
	return service.ListReleases(ctx, owner, repo, opts) //nolint:wrapcheck
}

func filterDraftReleases(releases []*RepositoryRelease) []*RepositoryRelease {
	arr := make([]*RepositoryRelease, 0, len(releases))
	for _, release := range releases {
		// Ignore draft releases
		if release.GetDraft() {
			continue
		}
		arr = append(arr, release)
	}
	return arr
}

// PullRequestsServiceImpl wraps PullRequestsService with GHES support.
type PullRequestsServiceImpl struct {
	defaultPRService PullRequestsService
	ghesPRService    PullRequestsService
}

// SetServices sets the default and GHES PullRequestsService.
func (p *PullRequestsServiceImpl) SetServices(defaultService, ghesService PullRequestsService) {
	p.defaultPRService = defaultService
	p.ghesPRService = ghesService
}

// CreateComment creates a pull request comment.
// If GHES is enabled, it always uses GHES (no fallback).
func (p *PullRequestsServiceImpl) CreateComment(ctx context.Context, owner, repo string, number int, comment *github.PullRequestComment) (*github.PullRequestComment, *github.Response, error) {
	if p.ghesPRService != nil {
		return p.ghesPRService.CreateComment(ctx, owner, repo, number, comment) //nolint:wrapcheck
	}
	return p.defaultPRService.CreateComment(ctx, owner, repo, number, comment) //nolint:wrapcheck
}
