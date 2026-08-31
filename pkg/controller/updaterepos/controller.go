// Package updaterepos updates GitHub Actions pins across GitHub repositories.
package updaterepos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	ghapi "github.com/google/go-github/v90/github"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/di"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/github"
)

const (
	defaultBranch  = "pinact/update-actions"
	defaultPRTitle = "chore: pin GitHub Actions"
	// DefaultConcurrency is the default number of repositories processed in parallel.
	DefaultConcurrency = 4
	orgReposPerPage    = 100
	gitAskpassPerm     = 0o700
	statusChanged      = "changed"
	statusSkipped      = "skipped"
	statusUnchanged    = "unchanged"
	statusFailed       = "failed"
)

type Param struct {
	Organizations       []string
	Repositories        []string
	IncludeRepositories []*regexp.Regexp
	ExcludeRepositories []*regexp.Regexp
	IncludeForks        bool
	IncludeArchived     bool
	Branch              string
	BaseBranch          string
	PRTitle             string
	PRBody              string
	Draft               bool
	DryRun              bool
	KeepWorkdir         bool
	Concurrency         int
	Update              bool
	IncludeActions      []*regexp.Regexp
	ExcludeActions      []*regexp.Regexp
	BranchToTags        []*regexp.Regexp
	MinAge              int
	Format              string
	Token               string
	GHESToken           string
	Flags               *di.Flags
}

type Result struct {
	Repository string `json:"repository"`
	Status     string `json:"status"`
	PRURL      string `json:"pr_url,omitempty"`
	Workdir    string `json:"workdir,omitempty"`
	Error      string `json:"error,omitempty"`
}

type Controller struct {
	client  *github.Client
	service GitHubService
	git     GitRunner
	pinner  Pinner
	param   Param
	stdout  io.Writer
	stderr  io.Writer
}

func New(client *github.Client, param Param, stdout, stderr io.Writer) *Controller {
	if param.Branch == "" {
		param.Branch = defaultBranch
	}
	if param.PRTitle == "" {
		param.PRTitle = defaultPRTitle
	}
	if param.PRBody == "" {
		param.PRBody = "Automated GitHub Actions pinning by pinact."
	}
	if param.Concurrency == 0 {
		param.Concurrency = DefaultConcurrency
	}
	controller := &Controller{client: client, service: newGitHubService(client), param: param, stdout: stdout, stderr: stderr}
	controller.git = localGitRunner{controller: controller}
	controller.pinner = controllerPinner{controller: controller}
	return controller
}

func newController(service GitHubService, git GitRunner, pinner Pinner, param Param, stdout io.Writer) *Controller {
	controller := New(nil, param, stdout, nil)
	controller.service = service
	if git != nil {
		controller.git = git
	}
	if pinner != nil {
		controller.pinner = pinner
	}
	return controller
}

func (c *Controller) Run(ctx context.Context, logger *slog.Logger) ([]Result, error) {
	repositories, err := c.repositories(ctx)
	if err != nil {
		return nil, err
	}
	jobs := make(chan *github.Repository)
	results := make(chan Result, len(repositories))
	var wg sync.WaitGroup
	for range c.param.Concurrency {
		wg.Go(func() {
			for repository := range jobs {
				results <- c.process(ctx, logger, repository)
			}
		})
	}
	go func() {
		for _, repository := range repositories {
			jobs <- repository
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()
	all := make([]Result, 0, len(repositories))
	failed := false
	for result := range results {
		all = append(all, result)
		if result.Status == statusFailed {
			failed = true
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Repository < all[j].Repository })
	c.output(all)
	if failed {
		return all, errors.New("one or more repositories failed")
	}
	return all, nil
}

func (c *Controller) repositories(ctx context.Context) ([]*github.Repository, error) {
	seen := map[string]*github.Repository{}
	explicit := map[string]bool{}
	if err := c.addExplicitRepositories(ctx, seen, explicit); err != nil {
		return nil, err
	}
	if err := c.addOrganizationRepositories(ctx, seen); err != nil {
		return nil, err
	}
	return c.filterRepositories(seen, explicit), nil
}

func (c *Controller) addExplicitRepositories(ctx context.Context, seen map[string]*github.Repository, explicit map[string]bool) error {
	for _, fullName := range c.param.Repositories {
		owner, name, err := parseRepoFullName(fullName)
		if err != nil {
			return err
		}
		repository, err := c.service.GetRepository(ctx, owner, name)
		if err != nil {
			return fmt.Errorf("get repository %s: %w", fullName, err)
		}
		seen[repository.GetFullName()] = repository
		explicit[repository.GetFullName()] = true
	}
	return nil
}

func parseRepoFullName(fullName string) (string, string, error) {
	owner, name, ok := strings.Cut(fullName, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return "", "", fmt.Errorf("repository must be in owner/name form: %s", fullName)
	}
	return owner, name, nil
}

func (c *Controller) addOrganizationRepositories(ctx context.Context, seen map[string]*github.Repository) error {
	for _, org := range c.param.Organizations {
		if err := c.listOrganizationRepositories(ctx, org, seen); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) listOrganizationRepositories(ctx context.Context, org string, seen map[string]*github.Repository) error {
	page := 0
	for {
		repositories, nextPage, err := c.service.ListOrganizationRepositories(ctx, org, page)
		if err != nil {
			return fmt.Errorf("list repositories for organization %s: %w", org, err)
		}
		for _, repository := range repositories {
			seen[repository.GetFullName()] = repository
		}
		if nextPage == 0 {
			return nil
		}
		page = nextPage
	}
}

func (c *Controller) filterRepositories(seen map[string]*github.Repository, explicit map[string]bool) []*github.Repository {
	result := make([]*github.Repository, 0, len(seen))
	for _, repository := range seen {
		name := repository.GetFullName()
		if c.excluded(name) {
			continue
		}
		if !explicit[name] && !c.selected(repository) {
			continue
		}
		result = append(result, repository)
	}
	return result
}

func (c *Controller) selected(repository *github.Repository) bool {
	if repository.GetArchived() && !c.param.IncludeArchived {
		return false
	}
	if repository.GetFork() && !c.param.IncludeForks {
		return false
	}
	name := repository.GetFullName()
	if c.excluded(name) {
		return false
	}
	if len(c.param.IncludeRepositories) == 0 {
		return true
	}
	for _, pattern := range c.param.IncludeRepositories {
		if pattern.MatchString(name) {
			return true
		}
	}
	return false
}

func (c *Controller) excluded(name string) bool {
	for _, pattern := range c.param.ExcludeRepositories {
		if pattern.MatchString(name) {
			return true
		}
	}
	return false
}

func (c *Controller) process(ctx context.Context, logger *slog.Logger, repository *github.Repository) Result {
	result := Result{Repository: repository.GetFullName()}
	workdir, cleanup, err := c.makeWorkdir()
	if err != nil {
		return c.fail(result, err)
	}
	defer cleanup()
	if c.param.KeepWorkdir {
		result.Workdir = workdir
	}
	base, err := c.cloneAndCheckoutBase(ctx, repository, workdir)
	if err != nil {
		return c.fail(result, err)
	}
	return c.syncRepository(ctx, logger, repository, workdir, base, result)
}

func (c *Controller) makeWorkdir() (string, func(), error) {
	workdir, err := os.MkdirTemp("", "pinact-update-repos-")
	if err != nil {
		return "", nil, fmt.Errorf("create temporary directory: %w", err)
	}
	cleanup := func() {}
	if !c.param.KeepWorkdir {
		cleanup = func() { os.RemoveAll(workdir) }
	}
	return workdir, cleanup, nil
}

func (c *Controller) cloneAndCheckoutBase(ctx context.Context, repository *github.Repository, workdir string) (string, error) {
	if err := c.runGit(ctx, workdir, "clone", "--quiet", repository.GetCloneURL(), "."); err != nil {
		return "", err
	}
	base := repository.GetDefaultBranch()
	if c.param.BaseBranch != "" {
		base = c.param.BaseBranch
	}
	if err := c.runGit(ctx, workdir, "checkout", "--quiet", base); err != nil {
		return "", fmt.Errorf("checkout base branch %s: %w", base, err)
	}
	return base, nil
}

func (c *Controller) syncRepository(ctx context.Context, logger *slog.Logger, repository *github.Repository, workdir, base string, result Result) Result {
	branchExists, err := c.branchExists(ctx, workdir)
	if err != nil {
		return c.fail(result, err)
	}
	status, err := c.pinStatus(ctx, logger, workdir)
	if err != nil {
		return c.fail(result, err)
	}
	if status != statusChanged {
		result.Status = status
		return result
	}
	if c.param.DryRun {
		result.Status = statusChanged
		return result
	}
	prURL, status, err := c.commitAndOpenPR(ctx, logger, repository, workdir, base, branchExists)
	if err != nil {
		return c.fail(result, err)
	}
	result.Status = status
	result.PRURL = prURL
	return result
}

func (c *Controller) pinStatus(ctx context.Context, logger *slog.Logger, workdir string) (string, error) {
	hasFiles, err := c.pinner.Pin(ctx, logger, workdir)
	if err != nil {
		return "", fmt.Errorf("pin repository: %w", err)
	}
	if !hasFiles {
		return statusSkipped, nil
	}
	changed, err := c.git.Changed(ctx, workdir)
	if err != nil {
		return "", fmt.Errorf("check repository changes: %w", err)
	}
	if !changed {
		return statusUnchanged, nil
	}
	return statusChanged, nil
}

func (c *Controller) commitAndOpenPR(ctx context.Context, logger *slog.Logger, repository *github.Repository, workdir, base string, branchExists bool) (string, string, error) {
	if err := c.runGit(ctx, workdir, "reset", "--hard"); err != nil {
		return "", "", err
	}
	existingPR, err := c.existingPullRequest(ctx, repository, base)
	if err != nil {
		return "", "", err
	}
	if err := c.checkoutBranch(ctx, workdir, base, branchExists); err != nil {
		return "", "", err
	}
	status, err := c.pinStatus(ctx, logger, workdir)
	if err != nil {
		return "", "", err
	}
	if status != statusChanged {
		return existingPRURL(existingPR), status, nil
	}
	if err := c.commitChanges(ctx, repository, workdir, base, branchExists); err != nil {
		return "", "", err
	}
	pr, err := c.pullRequest(ctx, repository, base, existingPR)
	if err != nil {
		return "", "", err
	}
	return pr.GetHTMLURL(), statusChanged, nil
}

func existingPRURL(pr *ghapi.PullRequest) string {
	if pr == nil {
		return ""
	}
	return pr.GetHTMLURL()
}

// commitChanges creates the commit through GitHub's Git Data API. When the
// token is an installation token, GitHub signs the commit as the App as long
// as author and committer are omitted.
func (c *Controller) commitChanges(ctx context.Context, repository *github.Repository, workdir, base string, branchExists bool) error {
	owner, repo := repository.GetOwner().GetLogin(), repository.GetName()
	parentSHA, parentTreeSHA, err := c.parentCommit(ctx, owner, repo, base, branchExists)
	if err != nil {
		return err
	}
	tree, err := c.createChangedTree(ctx, owner, repo, workdir, parentTreeSHA)
	if err != nil {
		return err
	}
	commit, err := c.service.CreateCommit(ctx, owner, repo, ghapi.Commit{
		Message: github.Ptr(c.param.PRTitle),
		Tree:    tree,
		Parents: []*ghapi.Commit{{SHA: github.Ptr(parentSHA)}},
	})
	if err != nil {
		return fmt.Errorf("create commit: %w", err)
	}
	if !commit.GetVerification().GetVerified() {
		return fmt.Errorf("GitHub did not verify the App commit: %s", commit.GetVerification().GetReason())
	}
	return c.publishBranch(ctx, owner, repo, commit.GetSHA(), branchExists)
}

func (c *Controller) parentCommit(ctx context.Context, owner, repo, base string, branchExists bool) (string, string, error) {
	parentBranch := base
	if branchExists {
		parentBranch = c.param.Branch
	}
	ref, err := c.service.GetRef(ctx, owner, repo, "heads/"+parentBranch)
	if err != nil {
		return "", "", fmt.Errorf("get branch %s: %w", parentBranch, err)
	}
	parentSHA := ref.GetObject().GetSHA()
	parent, err := c.service.GetCommit(ctx, owner, repo, parentSHA)
	if err != nil {
		return "", "", fmt.Errorf("get parent commit: %w", err)
	}
	return parentSHA, parent.GetTree().GetSHA(), nil
}

func (c *Controller) createChangedTree(ctx context.Context, owner, repo, workdir, parentTreeSHA string) (*ghapi.Tree, error) {
	paths, err := c.git.ChangedFiles(ctx, workdir)
	if err != nil {
		return nil, fmt.Errorf("list changed files: %w", err)
	}
	entries := make([]*ghapi.TreeEntry, 0, len(paths))
	for _, path := range paths {
		entry, err := c.blobEntry(ctx, owner, repo, workdir, path)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	tree, err := c.service.CreateTree(ctx, owner, repo, parentTreeSHA, entries)
	if err != nil {
		return nil, fmt.Errorf("create tree: %w", err)
	}
	return tree, nil
}

func (c *Controller) blobEntry(ctx context.Context, owner, repo, workdir, path string) (*ghapi.TreeEntry, error) {
	content, err := os.ReadFile(filepath.Join(workdir, path))
	if err != nil {
		return nil, fmt.Errorf("read changed file %s: %w", path, err)
	}
	blob, err := c.service.CreateBlob(ctx, owner, repo, ghapi.Blob{
		Content:  github.Ptr(string(content)),
		Encoding: github.Ptr("utf-8"),
	})
	if err != nil {
		return nil, fmt.Errorf("create blob for %s: %w", path, err)
	}
	return &ghapi.TreeEntry{
		Path: github.Ptr(path),
		Mode: github.Ptr("100644"),
		Type: github.Ptr("blob"),
		SHA:  github.Ptr(blob.GetSHA()),
	}, nil
}

func (c *Controller) publishBranch(ctx context.Context, owner, repo, sha string, branchExists bool) error {
	var err error
	if branchExists {
		err = c.service.UpdateRef(ctx, owner, repo, "heads/"+c.param.Branch, ghapi.UpdateRef{
			SHA:   sha,
			Force: github.Ptr(false),
		})
	} else {
		err = c.service.CreateRef(ctx, owner, repo, ghapi.CreateRef{
			Ref: "refs/heads/" + c.param.Branch,
			SHA: sha,
		})
	}
	if err != nil {
		return fmt.Errorf("update branch %s: %w", c.param.Branch, err)
	}
	return nil
}

func (c *Controller) pin(ctx context.Context, logger *slog.Logger, workdir string) (bool, error) {
	cfg, configPath, err := readConfig(workdir)
	if err != nil {
		return false, err
	}
	services, err := di.SetupGHESServices(ctx, c.client, cfg, c.param.Flags, c.param.GHESToken)
	if err != nil {
		return false, fmt.Errorf("set up GitHub services: %w", err)
	}
	files, err := workflowFiles(workdir, cfg, configPath)
	if err != nil {
		return false, err
	}
	if len(files) == 0 {
		return false, nil
	}
	ctrl := run.New(services.RepoService, services.GitService, afero.NewOsFs(), cfg, &run.ParamRun{
		WorkflowFilePaths: files, ConfigFilePath: configPath, CWD: workdir, Fix: true, Update: c.param.Update,
		Stderr: c.stderr, Stdout: c.stdout, Includes: c.param.IncludeActions, Excludes: c.param.ExcludeActions,
		BranchToTags: c.param.BranchToTags, MinAge: c.param.MinAge,
		Now: time.Now(),
	})
	if err := ctrl.Run(ctx, logger); err != nil {
		return true, fmt.Errorf("pin actions: %w", err)
	}
	return true, nil
}

func readConfig(workdir string) (*config.Config, string, error) {
	fs := afero.NewOsFs()
	configPath, err := findConfigPath(workdir)
	if err != nil {
		return nil, "", err
	}
	globalPath, err := config.NewFinder(fs).FindGlobal()
	if err != nil {
		return nil, "", fmt.Errorf("find global configuration: %w", err)
	}
	cfg := &config.Config{Separator: " # "}
	if configPath == "" && globalPath == "" {
		return cfg, configPath, nil
	}
	if err := config.NewReader(fs).ReadAndMerge(cfg, configPath, globalPath); err != nil {
		return nil, "", fmt.Errorf("read configuration: %w", err)
	}
	if cfg.Separator == "" {
		cfg.Separator = " # "
	}
	return cfg, configPath, nil
}

func findConfigPath(workdir string) (string, error) {
	paths := []string{".pinact.yaml", ".github/pinact.yaml", ".pinact.yml", ".github/pinact.yml"}
	for _, path := range paths {
		candidate := filepath.Join(workdir, path)
		_, err := os.Stat(candidate)
		if err == nil {
			return candidate, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat config file %s: %w", path, err)
		}
	}
	return "", nil
}

func workflowFiles(workdir string, cfg *config.Config, configPath string) ([]string, error) {
	if len(cfg.Files) > 0 {
		return globConfigFiles(workdir, cfg, configPath)
	}
	patterns := []string{
		".github/workflows/*.yml", ".github/workflows/*.yaml",
		"action.yml", "action.yaml",
		"*/action.yml", "*/action.yaml",
		"*/*/action.yml", "*/*/action.yaml",
		"*/*/*/action.yml", "*/*/*/action.yaml",
	}
	return globPatterns(workdir, patterns)
}

func globConfigFiles(workdir string, cfg *config.Config, configPath string) ([]string, error) {
	files := make([]string, 0, len(cfg.Files))
	for _, file := range cfg.Files {
		base := workdir
		if configPath != "" {
			base = filepath.Dir(configPath)
		}
		matches, err := filepath.Glob(filepath.Join(base, file.Pattern))
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", file.Pattern, err)
		}
		files = append(files, matches...)
	}
	return files, nil
}

func globPatterns(workdir string, patterns []string) ([]string, error) {
	var files []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(workdir, pattern))
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", pattern, err)
		}
		files = append(files, matches...)
	}
	return files, nil
}

func (c *Controller) branchExists(ctx context.Context, workdir string) (bool, error) {
	err := c.git.Run(ctx, workdir, io.Discard, io.Discard, "ls-remote", "--exit-code", "--heads", "origin", c.param.Branch)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 2 {
		return false, nil
	}
	return false, fmt.Errorf("check branch %s: %w", c.param.Branch, err)
}

func (c *Controller) checkoutBranch(ctx context.Context, workdir, base string, branchExists bool) error {
	if branchExists {
		if err := c.runGit(ctx, workdir, "fetch", "--quiet", "origin", c.param.Branch); err != nil {
			return fmt.Errorf("fetch branch %s: %w", c.param.Branch, err)
		}
		return c.runGit(ctx, workdir, "checkout", "--quiet", "-B", c.param.Branch, "origin/"+c.param.Branch)
	}
	return c.runGit(ctx, workdir, "checkout", "--quiet", "-b", c.param.Branch, base)
}

func (c *Controller) existingPullRequest(ctx context.Context, repository *github.Repository, base string) (*ghapi.PullRequest, error) {
	prs, err := c.service.ListOpenPullRequests(ctx, repository.GetOwner().GetLogin(), repository.GetName(), repository.GetOwner().GetLogin()+":"+c.param.Branch)
	if err != nil {
		return nil, fmt.Errorf("find existing pull request: %w", err)
	}
	if len(prs) == 0 {
		return nil, nil //nolint:nilnil // no open pull request is a valid result
	}
	if prs[0].GetBase().GetRef() != base {
		return nil, fmt.Errorf("existing pull request targets %s instead of %s", prs[0].GetBase().GetRef(), base)
	}
	return prs[0], nil
}

func (c *Controller) pullRequest(ctx context.Context, repository *github.Repository, base string, existing *ghapi.PullRequest) (*ghapi.PullRequest, error) {
	if existing != nil {
		return existing, nil
	}
	pr, err := c.service.CreatePullRequest(ctx, repository.GetOwner().GetLogin(), repository.GetName(), ghapi.CreatePullRequest{
		Title: github.Ptr(c.param.PRTitle),
		Head:  c.param.Branch,
		Base:  base,
		Body:  github.Ptr(c.param.PRBody),
		Draft: github.Ptr(c.param.Draft),
	})
	if err != nil {
		return nil, fmt.Errorf("create pull request: %w", err)
	}
	return pr, nil
}

func (c *Controller) runGit(ctx context.Context, dir string, args ...string) error {
	if err := c.git.Run(ctx, dir, c.stdout, c.stderr, args...); err != nil {
		return fmt.Errorf("run git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (c *Controller) executeGit(ctx context.Context, dir string, stdout, stderr io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stderr = stderr
	cmd.Stdout = stdout
	if c.param.Token != "" {
		path, err := c.writeGitAskpass()
		if err != nil {
			return err
		}
		defer os.Remove(path)
		cmd.Env = append(os.Environ(), "GIT_ASKPASS="+path, "GIT_TERMINAL_PROMPT=0", "PINACT_GIT_TOKEN="+c.param.Token)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run git %s: %w", strings.Join(args, " "), err)
	}
	return nil
}

func (c *Controller) writeGitAskpass() (string, error) {
	askpass, err := os.CreateTemp("", "pinact-git-askpass-")
	if err != nil {
		return "", fmt.Errorf("create Git credential helper: %w", err)
	}
	path := askpass.Name()
	if _, err := askpass.WriteString("#!/bin/sh\ncase \"$1\" in *Username*) printf '%s\\n' x-access-token ;; *) printf '%s\\n' \"$PINACT_GIT_TOKEN\" ;; esac\n"); err != nil {
		askpass.Close()
		os.Remove(path)
		return "", fmt.Errorf("write Git credential helper: %w", err)
	}
	if err := askpass.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close Git credential helper: %w", err)
	}
	if err := os.Chmod(path, gitAskpassPerm); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("make Git credential helper executable: %w", err)
	}
	return path, nil
}

func (c *Controller) fail(result Result, err error) Result {
	result.Status = statusFailed
	result.Error = err.Error()
	return result
}

func (c *Controller) output(results []Result) {
	if c.param.Format == "json" {
		if err := json.NewEncoder(c.stdout).Encode(results); err != nil {
			fmt.Fprintf(c.stderr, "encode JSON results: %v\n", err)
		}
		return
	}
	for _, result := range results {
		switch {
		case result.Error != "":
			fmt.Fprintf(c.stdout, "%s: %s: %s\n", result.Repository, result.Status, result.Error)
		case result.PRURL != "":
			fmt.Fprintf(c.stdout, "%s: %s: %s\n", result.Repository, result.Status, result.PRURL)
		default:
			fmt.Fprintf(c.stdout, "%s: %s\n", result.Repository, result.Status)
		}
	}
}
