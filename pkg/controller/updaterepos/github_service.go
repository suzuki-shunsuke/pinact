package updaterepos

import (
	"context"
	"fmt"

	ghapi "github.com/google/go-github/v90/github"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/github"
)

// GitHubService contains only the GitHub operations update-repos orchestrates.
// Keeping this surface narrow makes the controller independently testable.
//
//nolint:interfacebloat // GitHub's Git Data workflow requires all of these operations.
type GitHubService interface {
	GetRepository(ctx context.Context, owner, repo string) (*github.Repository, error)
	ListOrganizationRepositories(ctx context.Context, org string, page int) ([]*github.Repository, int, error)
	GetRef(ctx context.Context, owner, repo, branch string) (*ghapi.Reference, error)
	GetCommit(ctx context.Context, owner, repo, sha string) (*ghapi.Commit, error)
	CreateBlob(ctx context.Context, owner, repo string, blob ghapi.Blob) (*ghapi.Blob, error)
	CreateTree(ctx context.Context, owner, repo, baseTree string, entries []*ghapi.TreeEntry) (*ghapi.Tree, error)
	CreateCommit(ctx context.Context, owner, repo string, commit ghapi.Commit) (*ghapi.Commit, error)
	CreateRef(ctx context.Context, owner, repo string, ref ghapi.CreateRef) error
	UpdateRef(ctx context.Context, owner, repo, branch string, ref ghapi.UpdateRef) error
	ListOpenPullRequests(ctx context.Context, owner, repo, head string) ([]*ghapi.PullRequest, error)
	CreatePullRequest(ctx context.Context, owner, repo string, pr ghapi.CreatePullRequest) (*ghapi.PullRequest, error)
}

type githubService struct{ client *github.Client }

func newGitHubService(client *github.Client) GitHubService { return &githubService{client: client} }

func (s *githubService) GetRepository(ctx context.Context, owner, repo string) (*github.Repository, error) {
	repository, _, err := s.client.Repositories.Get(ctx, owner, repo)
	return repository, githubError("get repository", err)
}

func (s *githubService) ListOrganizationRepositories(ctx context.Context, org string, page int) ([]*github.Repository, int, error) {
	repositories, response, err := s.client.Repositories.ListByOrg(ctx, org, &ghapi.RepositoryListByOrgOptions{ListOptions: ghapi.ListOptions{Page: page, PerPage: orgReposPerPage}, Type: "all"})
	if err != nil {
		return nil, 0, githubError("list organization repositories", err)
	}
	return repositories, response.NextPage, nil
}

func (s *githubService) GetRef(ctx context.Context, owner, repo, branch string) (*ghapi.Reference, error) {
	ref, _, err := s.client.Git.GetRef(ctx, owner, repo, branch)
	return ref, githubError("get ref", err)
}

func (s *githubService) GetCommit(ctx context.Context, owner, repo, sha string) (*ghapi.Commit, error) {
	commit, _, err := s.client.Git.GetCommit(ctx, owner, repo, sha)
	return commit, githubError("get commit", err)
}

func (s *githubService) CreateBlob(ctx context.Context, owner, repo string, blob ghapi.Blob) (*ghapi.Blob, error) {
	created, _, err := s.client.Git.CreateBlob(ctx, owner, repo, blob)
	return created, githubError("create blob", err)
}

func (s *githubService) CreateTree(ctx context.Context, owner, repo, baseTree string, entries []*ghapi.TreeEntry) (*ghapi.Tree, error) {
	tree, _, err := s.client.Git.CreateTree(ctx, owner, repo, baseTree, entries)
	return tree, githubError("create tree", err)
}

func (s *githubService) CreateCommit(ctx context.Context, owner, repo string, commit ghapi.Commit) (*ghapi.Commit, error) {
	created, _, err := s.client.Git.CreateCommit(ctx, owner, repo, commit, nil)
	return created, githubError("create commit", err)
}

func (s *githubService) CreateRef(ctx context.Context, owner, repo string, ref ghapi.CreateRef) error {
	_, _, err := s.client.Git.CreateRef(ctx, owner, repo, ref)
	return githubError("create ref", err)
}

func (s *githubService) UpdateRef(ctx context.Context, owner, repo, branch string, ref ghapi.UpdateRef) error {
	_, _, err := s.client.Git.UpdateRef(ctx, owner, repo, branch, ref)
	return githubError("update ref", err)
}

func (s *githubService) ListOpenPullRequests(ctx context.Context, owner, repo, head string) ([]*ghapi.PullRequest, error) {
	prs, _, err := s.client.PullRequests.List(ctx, owner, repo, &ghapi.PullRequestListOptions{Head: head, State: "open"})
	return prs, githubError("list pull requests", err)
}

func (s *githubService) CreatePullRequest(ctx context.Context, owner, repo string, pr ghapi.CreatePullRequest) (*ghapi.PullRequest, error) {
	created, _, err := s.client.PullRequests.Create(ctx, owner, repo, pr)
	return created, githubError("create pull request", err)
}

func githubError(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}
