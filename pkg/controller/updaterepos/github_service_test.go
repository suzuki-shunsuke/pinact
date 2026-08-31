package updaterepos //nolint:testpackage // tests unexported helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	ghapi "github.com/google/go-github/v90/github"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/github"
)

type fakeGitHubService struct {
	commit          *ghapi.Commit
	createdCommit   ghapi.Commit
	createdRef      *ghapi.CreateRef
	updatedRef      *ghapi.UpdateRef
	createCommitErr error
	openPRs         []*ghapi.PullRequest
	createdPR       *ghapi.CreatePullRequest
	repository      *github.Repository
	organization    []*github.Repository
}

func (f *fakeGitHubService) GetRepository(context.Context, string, string) (*github.Repository, error) {
	return f.repository, nil
}

func (f *fakeGitHubService) ListOrganizationRepositories(context.Context, string, int) ([]*github.Repository, int, error) {
	return f.organization, 0, nil
}

func (f *fakeGitHubService) GetRef(context.Context, string, string, string) (*ghapi.Reference, error) {
	return &ghapi.Reference{Object: &ghapi.GitObject{SHA: github.Ptr("parent-sha")}}, nil
}

func (f *fakeGitHubService) GetCommit(context.Context, string, string, string) (*ghapi.Commit, error) {
	return &ghapi.Commit{Tree: &ghapi.Tree{SHA: github.Ptr("parent-tree")}}, nil
}

func (f *fakeGitHubService) CreateBlob(_ context.Context, _ string, _ string, blob ghapi.Blob) (*ghapi.Blob, error) {
	if blob.GetContent() != "updated\n" || blob.GetEncoding() != "utf-8" {
		return nil, errors.New("unexpected blob")
	}
	return &ghapi.Blob{SHA: github.Ptr("blob-sha")}, nil
}

func (f *fakeGitHubService) CreateTree(_ context.Context, _ string, _ string, baseTree string, entries []*ghapi.TreeEntry) (*ghapi.Tree, error) {
	if baseTree != "parent-tree" || len(entries) != 1 || entries[0].GetPath() != "workflow.yml" || entries[0].GetSHA() != "blob-sha" {
		return nil, errors.New("unexpected tree")
	}
	return &ghapi.Tree{SHA: github.Ptr("new-tree")}, nil
}

func (f *fakeGitHubService) CreateCommit(_ context.Context, _ string, _ string, commit ghapi.Commit) (*ghapi.Commit, error) {
	f.createdCommit = commit
	if f.createCommitErr != nil {
		return nil, f.createCommitErr
	}
	return f.commit, nil
}

func (f *fakeGitHubService) CreateRef(_ context.Context, _ string, _ string, ref ghapi.CreateRef) error {
	f.createdRef = &ref
	return nil
}

func (f *fakeGitHubService) UpdateRef(_ context.Context, _ string, _ string, _ string, ref ghapi.UpdateRef) error {
	f.updatedRef = &ref
	return nil
}

func (f *fakeGitHubService) ListOpenPullRequests(context.Context, string, string, string) ([]*ghapi.PullRequest, error) {
	return f.openPRs, nil
}

func (f *fakeGitHubService) CreatePullRequest(_ context.Context, _ string, _ string, pr ghapi.CreatePullRequest) (*ghapi.PullRequest, error) {
	f.createdPR = &pr
	return &ghapi.PullRequest{HTMLURL: github.Ptr("https://example.com/pr/1")}, nil
}

func TestCommitChangesCreatesVerifiedAppCommit(t *testing.T) {
	t.Parallel()
	workdir := changedGitWorkdir(t)
	service := &fakeGitHubService{commit: &ghapi.Commit{
		SHA:          github.Ptr("new-commit"),
		Verification: &ghapi.SignatureVerification{Verified: github.Ptr(true)},
	}}
	controller := newController(service, nil, nil, Param{PRTitle: defaultPRTitle}, nil)
	repository := &github.Repository{Owner: &ghapi.User{Login: github.Ptr("acme")}, Name: github.Ptr("service")}
	if err := controller.commitChanges(context.Background(), repository, workdir, "main", false); err != nil {
		t.Fatal(err)
	}
	if service.createdCommit.GetAuthor() != nil || service.createdCommit.GetCommitter() != nil {
		t.Fatal("CreateCommit received a custom author or committer")
	}
	if service.createdCommit.GetMessage() != "chore: pin GitHub Actions" || service.createdCommit.GetTree().GetSHA() != "new-tree" {
		t.Fatalf("unexpected commit: %#v", service.createdCommit)
	}
	if service.createdRef == nil || service.createdRef.GetRef() != "refs/heads/pinact/update-actions" || service.createdRef.GetSHA() != "new-commit" {
		t.Fatalf("unexpected created ref: %#v", service.createdRef)
	}
}

func TestCommitChangesRejectsUnverifiedCommit(t *testing.T) {
	t.Parallel()
	workdir := changedGitWorkdir(t)
	service := &fakeGitHubService{commit: &ghapi.Commit{
		SHA:          github.Ptr("new-commit"),
		Verification: &ghapi.SignatureVerification{Verified: github.Ptr(false), Reason: github.Ptr("unsigned")},
	}}
	controller := newController(service, nil, nil, Param{}, nil)
	repository := &github.Repository{Owner: &ghapi.User{Login: github.Ptr("acme")}, Name: github.Ptr("service")}
	if err := controller.commitChanges(context.Background(), repository, workdir, "main", false); err == nil {
		t.Fatal("commitChanges succeeded for an unverified commit")
	}
	if service.createdRef != nil || service.updatedRef != nil {
		t.Fatal("commitChanges published an unverified commit")
	}
}

func TestCommitChangesUpdatesExistingBranch(t *testing.T) {
	t.Parallel()
	workdir := changedGitWorkdir(t)
	service := &fakeGitHubService{commit: &ghapi.Commit{
		SHA:          github.Ptr("new-commit"),
		Verification: &ghapi.SignatureVerification{Verified: github.Ptr(true)},
	}}
	controller := newController(service, nil, nil, Param{}, nil)
	repository := &github.Repository{Owner: &ghapi.User{Login: github.Ptr("acme")}, Name: github.Ptr("service")}
	if err := controller.commitChanges(context.Background(), repository, workdir, "main", true); err != nil {
		t.Fatal(err)
	}
	if service.createdRef != nil || service.updatedRef == nil || service.updatedRef.GetSHA() != "new-commit" || service.updatedRef.GetForce() {
		t.Fatalf("unexpected ref update: create=%#v update=%#v", service.createdRef, service.updatedRef)
	}
}

func TestPullRequestCreatesExpectedRequest(t *testing.T) {
	t.Parallel()
	service := &fakeGitHubService{}
	controller := newController(service, nil, nil, Param{Branch: "pinact/custom", PRTitle: "title", PRBody: "body", Draft: true}, nil)
	repository := &github.Repository{Owner: &ghapi.User{Login: github.Ptr("acme")}, Name: github.Ptr("service")}
	pr, err := controller.pullRequest(context.Background(), repository, "main", nil)
	if err != nil {
		t.Fatal(err)
	}
	if pr.GetHTMLURL() != "https://example.com/pr/1" || service.createdPR == nil {
		t.Fatalf("unexpected pull request: %#v", pr)
	}
	if service.createdPR.GetHead() != "pinact/custom" || service.createdPR.GetBase() != "main" || service.createdPR.GetTitle() != "title" || service.createdPR.GetBody() != "body" || !service.createdPR.GetDraft() {
		t.Fatalf("unexpected pull request input: %#v", service.createdPR)
	}
}

func TestExistingPullRequestRejectsDifferentBase(t *testing.T) {
	t.Parallel()
	service := &fakeGitHubService{openPRs: []*ghapi.PullRequest{{Base: &ghapi.PullRequestBranch{Ref: github.Ptr("release")}}}}
	controller := newController(service, nil, nil, Param{}, nil)
	repository := &github.Repository{Owner: &ghapi.User{Login: github.Ptr("acme")}, Name: github.Ptr("service")}
	if _, err := controller.existingPullRequest(context.Background(), repository, "main"); err == nil {
		t.Fatal("existingPullRequest accepted a pull request with a different base")
	}
}

type fakeGitRunner struct {
	changed bool
	calls   [][]string
}

func (r *fakeGitRunner) Run(_ context.Context, _ string, _ io.Writer, _ io.Writer, args ...string) error {
	r.calls = append(r.calls, args)
	return nil
}

func (r *fakeGitRunner) Changed(context.Context, string) (bool, error) { return r.changed, nil }

func (r *fakeGitRunner) ChangedFiles(context.Context, string) ([]string, error) { return nil, nil }

type fakePinner struct {
	hasFiles bool
	err      error
	calls    int
}

func (p *fakePinner) Pin(context.Context, *slog.Logger, string) (bool, error) {
	p.calls++
	return p.hasFiles, p.err
}

func TestSyncRepositoryDryRunDoesNotMutateGitHub(t *testing.T) {
	t.Parallel()
	service := &fakeGitHubService{}
	git := &fakeGitRunner{changed: true}
	pinner := &fakePinner{hasFiles: true}
	controller := newController(service, git, pinner, Param{DryRun: true}, nil)
	repository := &github.Repository{FullName: github.Ptr("acme/service")}
	result := controller.syncRepository(context.Background(), slog.Default(), repository, t.TempDir(), "main", Result{Repository: "acme/service"})
	if result.Status != statusChanged || pinner.calls != 1 || service.createdRef != nil || service.updatedRef != nil || service.createdPR != nil {
		t.Fatalf("unexpected dry-run result: %#v", result)
	}
}

func TestRunFiltersOrganizationRepositories(t *testing.T) {
	t.Parallel()
	service := &fakeGitHubService{organization: []*github.Repository{
		{FullName: github.Ptr("acme/active")},
		{FullName: github.Ptr("acme/fork"), Fork: github.Ptr(true)},
		{FullName: github.Ptr("acme/archived"), Archived: github.Ptr(true)},
	}}
	git := &fakeGitRunner{}
	pinner := &fakePinner{hasFiles: false}
	var stdout bytes.Buffer
	controller := newController(service, git, pinner, Param{Organizations: []string{"acme"}, Concurrency: 1, DryRun: true}, &stdout)
	results, err := controller.Run(context.Background(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Repository != "acme/active" || results[0].Status != statusSkipped || pinner.calls != 1 {
		t.Fatalf("unexpected results: %#v", results)
	}
	if stdout.String() != "acme/active: skipped\n" {
		t.Fatalf("unexpected output: %q", stdout.String())
	}
}

func TestGitHubServiceCreateCommitOmitsIdentity(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v3/repos/acme/service/git/commits" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if _, ok := body["author"]; ok {
			t.Fatal("commit request includes author")
		}
		if _, ok := body["committer"]; ok {
			t.Fatal("commit request includes committer")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sha":"commit","verification":{"verified":true}}`))
	}))
	defer server.Close()
	client, err := github.NewWithBaseURL(context.Background(), server.URL+"/api/v3/", "token")
	if err != nil {
		t.Fatal(err)
	}
	service := newGitHubService(client)
	commit, err := service.CreateCommit(context.Background(), "acme", "service", ghapi.Commit{Message: github.Ptr("message"), Tree: &ghapi.Tree{SHA: github.Ptr("tree")}})
	if err != nil {
		t.Fatal(err)
	}
	if !commit.GetVerification().GetVerified() {
		t.Fatal("commit response was not decoded")
	}
}

func changedGitWorkdir(t *testing.T) string {
	t.Helper()
	workdir := t.TempDir()
	runGit(t, workdir, "init", "--quiet")
	runGit(t, workdir, "config", "user.name", "test")
	runGit(t, workdir, "config", "user.email", "test@example.com")
	path := filepath.Join(workdir, "workflow.yml")
	if err := os.WriteFile(path, []byte("original\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, workdir, "add", "workflow.yml")
	runGit(t, workdir, "commit", "--quiet", "-m", "initial")
	if err := os.WriteFile(path, []byte("updated\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return workdir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
