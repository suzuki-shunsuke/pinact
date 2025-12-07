// Package run implements the 'pinact run' command, the core functionality of pinact.
// This package orchestrates the main pinning process for GitHub Actions and reusable workflows,
// including version resolution, SHA pinning, update operations, and pull request review creation.
// It handles various modes of operation (check, diff, fix, update, review) and integrates
// with GitHub Actions CI environment for automated processing. The package also manages
// include/exclude patterns for selective action processing and coordinates with the
// controller layer to perform the actual file modifications.
package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/flag"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	urfavecli "github.com/urfave/cli/v3"
)

type Flags struct {
	LogLevel  string
	Config    string
	Verify    bool
	Check     bool
	Update    bool
	Review    bool
	Fix       bool
	FixIsSet  bool
	Diff      bool
	RepoOwner string
	RepoName  string
	SHA       string
	PR        int
	Include   []string
	Exclude   []string
	Args      []string
}

// New creates a new run command for the CLI.
// It initializes a runner with the provided logger and returns
// the configured CLI command for pinning GitHub Actions versions.
//
// Parameters:
//   - logger: slog logger for structured logging
//   - logLevelVar: slog level variable for dynamic log level changes
//
// Returns a pointer to the configured CLI command.
func New(logger *slogutil.Logger, globalFlags *flag.GlobalFlags) *urfavecli.Command {
	r := &runner{}
	return r.Command(logger, globalFlags)
}

type runner struct{}

// Command builds and returns the run CLI command configuration.
// It defines all flags, options, and the action handler for the run subcommand.
// This command handles the core pinning functionality with various modes
// like check, diff, fix, update, and review.
//
// Returns a pointer to the configured CLI command.
func (r *runner) Command(logger *slogutil.Logger, globalFlags *flag.GlobalFlags) *urfavecli.Command { //nolint:funlen
	flags := &Flags{}
	return &urfavecli.Command{
		Name:  "run",
		Usage: "Pin GitHub Actions versions",
		Description: `If no argument is passed, pinact searches GitHub Actions workflow files from .github/workflows.

$ pinact run

You can also pass workflow file paths as arguments.

e.g.

$ pinact run .github/actions/foo/action.yaml .github/actions/bar/action.yaml
`,
		Action: func(ctx context.Context, cmd *urfavecli.Command) error {
			flags.LogLevel = globalFlags.LogLevel
			flags.Config = globalFlags.Config
			flags.FixIsSet = cmd.IsSet("fix")
			flags.Args = cmd.Args().Slice()
			return r.action(ctx, logger, flags)
		},
		Flags: []urfavecli.Flag{
			&urfavecli.BoolFlag{
				Name:        "verify",
				Aliases:     []string{"v"},
				Usage:       "Verify if pairs of commit SHA and version are correct",
				Destination: &flags.Verify,
			},
			&urfavecli.BoolFlag{
				Name:        "check",
				Usage:       "Exit with a non-zero status code if actions are not pinned. If this is true, files aren't updated",
				Destination: &flags.Check,
			},
			&urfavecli.BoolFlag{
				Name:        "update",
				Aliases:     []string{"u"},
				Usage:       "Update actions to latest versions",
				Destination: &flags.Update,
			},
			&urfavecli.BoolFlag{
				Name:        "review",
				Usage:       "Create reviews",
				Destination: &flags.Review,
			},
			&urfavecli.BoolFlag{
				Name:        "fix",
				Usage:       "Fix code. By default, this is true. If -check or -diff is true, this is false by default",
				Destination: &flags.Fix,
			},
			&urfavecli.BoolFlag{
				Name:        "diff",
				Usage:       "Output diff. By default, this is false",
				Destination: &flags.Diff,
			},
			&urfavecli.StringFlag{
				Name:        "repo-owner",
				Usage:       "GitHub repository owner",
				Sources:     urfavecli.EnvVars("GITHUB_REPOSITORY_OWNER"),
				Destination: &flags.RepoOwner,
			},
			&urfavecli.StringFlag{
				Name:        "repo-name",
				Usage:       "GitHub repository name",
				Destination: &flags.RepoName,
			},
			&urfavecli.StringFlag{
				Name:        "sha",
				Usage:       "Commit SHA to be reviewed",
				Destination: &flags.SHA,
			},
			&urfavecli.IntFlag{
				Name:        "pr",
				Usage:       "GitHub pull request number",
				Destination: &flags.PR,
			},
			&urfavecli.StringSliceFlag{
				Name:        "include",
				Aliases:     []string{"i"},
				Usage:       "A regular expression to fix actions",
				Destination: &flags.Include,
			},
			&urfavecli.StringSliceFlag{
				Name:        "exclude",
				Aliases:     []string{"e"},
				Usage:       "A regular expression to exclude actions",
				Destination: &flags.Exclude,
			},
		},
	}
}

type Event struct {
	PullRequest *PullRequest `json:"pull_request"`
	Issue       *Issue       `json:"issue"`
	Repository  *Repository  `json:"repository"`
}

// RepoName extracts the repository name from the GitHub event.
// It safely accesses the repository information from the event payload.
//
// Returns the repository name or empty string if not available.
func (e *Event) RepoName() string {
	if e != nil && e.Repository != nil {
		return e.Repository.Name
	}
	return ""
}

// PRNumber extracts the pull request or issue number from the GitHub event.
// It checks both pull request and issue fields to find the number.
//
// Returns the PR/issue number or 0 if not available.
func (e *Event) PRNumber() int {
	if e == nil {
		return 0
	}
	if e.PullRequest != nil {
		return e.PullRequest.Number
	}
	if e.Issue != nil {
		return e.Issue.Number
	}
	return 0
}

// SHA extracts the commit SHA from the GitHub event.
// It looks for the SHA in the pull request head information.
//
// Returns the commit SHA or empty string if not available.
func (e *Event) SHA() string {
	if e == nil {
		return ""
	}
	if e.PullRequest != nil && e.PullRequest.Head != nil {
		return e.PullRequest.Head.SHA
	}
	return ""
}

type Issue struct {
	Number int `json:"number"`
}

type PullRequest struct {
	Number int   `json:"number"`
	Head   *Head `json:"head"`
}

type Repository struct {
	Owner *Owner `json:"owner"`
	Name  string `json:"name"`
}

type Owner struct {
	Login string `json:"login"`
}

type Head struct {
	SHA string `json:"sha"`
}

// action executes the main run command logic.
// It configures logging, processes GitHub Actions context, parses includes/excludes,
// sets up the controller, and executes the pinning operation.
// Returns an error if the operation fails.
func (r *runner) action(ctx context.Context, logger *slogutil.Logger, flags *Flags) error { //nolint:cyclop,funlen
	isGitHubActions := os.Getenv("GITHUB_ACTIONS") == "true"
	if isGitHubActions {
		color.NoColor = false
	}
	if err := logger.SetLevel(flags.LogLevel); err != nil {
		return fmt.Errorf("set log level: %w", err)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get the current directory: %w", err)
	}

	gh := github.New(ctx, logger.Logger)
	fs := afero.NewOsFs()
	var review *run.Review
	if flags.Review {
		review = &run.Review{
			RepoOwner:   flags.RepoOwner,
			RepoName:    flags.RepoName,
			PullRequest: flags.PR,
			SHA:         flags.SHA,
		}
		if isGitHubActions {
			if err := r.setReview(fs, review); err != nil {
				slogerr.WithError(logger.Logger, err).Error("set review information")
			}
		}
		if !review.Valid() {
			logger.Warn("skip creating reviews because the review information is invalid")
			review = nil
		}
	}
	includes, err := parseIncludes(flags.Include)
	if err != nil {
		return err
	}
	excludes, err := parseExcludes(flags.Exclude)
	if err != nil {
		return err
	}
	param := &run.ParamRun{
		WorkflowFilePaths: flags.Args,
		ConfigFilePath:    flags.Config,
		PWD:               pwd,
		IsVerify:          flags.Verify,
		Check:             flags.Check,
		Update:            flags.Update,
		Diff:              flags.Diff,
		Fix:               true,
		IsGitHubActions:   isGitHubActions,
		Stderr:            os.Stderr,
		Review:            review,
		Includes:          includes,
		Excludes:          excludes,
	}
	if flags.FixIsSet {
		param.Fix = flags.Fix
	} else if param.Check || param.Diff {
		param.Fix = false
	}
	ctrl := run.New(&run.RepositoriesServiceImpl{
		Tags:                map[string]*run.ListTagsResult{},
		Releases:            map[string]*run.ListReleasesResult{},
		Commits:             map[string]*run.GetCommitSHA1Result{},
		RepositoriesService: gh.Repositories,
	}, gh.PullRequests, fs, config.NewFinder(fs), config.NewReader(fs), param)
	return ctrl.Run(ctx, logger.Logger) //nolint:wrapcheck
}

// parseIncludes compiles include regular expressions from command-line options.
// These patterns are used to filter which actions should be processed.
//
// Parameters:
//   - opts: slice of include pattern strings
//
// Returns compiled regular expressions or an error if compilation fails.
func parseIncludes(opts []string) ([]*regexp.Regexp, error) {
	includes := make([]*regexp.Regexp, len(opts))
	for i, include := range opts {
		r, err := regexp.Compile(include)
		if err != nil {
			return nil, fmt.Errorf("compile an include regexp: %w", slogerr.With(err, "regexp", include))
		}
		includes[i] = r
	}
	return includes, nil
}

// parseExcludes compiles exclude regular expressions from command-line options.
// These patterns are used to filter which actions should be skipped.
//
// Parameters:
//   - opts: slice of exclude pattern strings
//
// Returns compiled regular expressions or an error if compilation fails.
func parseExcludes(opts []string) ([]*regexp.Regexp, error) {
	excludes := make([]*regexp.Regexp, len(opts))
	for i, exclude := range opts {
		r, err := regexp.Compile(exclude)
		if err != nil {
			return nil, fmt.Errorf("compile an exclude regexp: %w", slogerr.With(err, "regexp", exclude))
		}
		excludes[i] = r
	}
	return excludes, nil
}

// setReview configures review information from GitHub Actions environment.
// It extracts repository name, pull request number, and commit SHA from
// environment variables and GitHub event payload.
//
// Parameters:
//   - fs: filesystem interface for reading event files
//   - review: review configuration to populate
//
// Returns an error if required information cannot be extracted.
func (r *runner) setReview(fs afero.Fs, review *run.Review) error {
	if review.RepoName == "" {
		repo := os.Getenv("GITHUB_REPOSITORY")
		_, repoName, ok := strings.Cut(repo, "/")
		if !ok {
			return fmt.Errorf("GITHUB_REPOSITORY is not set or invalid: %s", repo)
		}
		if repoName == "" {
			return fmt.Errorf("GITHUB_REPOSITORY is invalid: %s", repo)
		}
		review.RepoName = repoName
	}
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		return nil
	}
	var ev *Event
	if review.PullRequest == 0 {
		ev = &Event{}
		if err := r.readEvent(fs, ev, eventPath); err != nil {
			return err
		}
		review.PullRequest = ev.PRNumber()
	}
	if review.SHA != "" {
		return nil
	}
	if ev == nil {
		ev = &Event{}
		if err := r.readEvent(fs, ev, eventPath); err != nil {
			return err
		}
	}
	review.SHA = ev.SHA()
	return nil
}

// readEvent reads and parses the GitHub event payload from file.
// It unmarshals the JSON event data into the provided Event struct.
//
// Parameters:
//   - fs: filesystem interface for file operations
//   - ev: Event struct to populate with parsed data
//   - eventPath: path to the GitHub event file
//
// Returns an error if the file cannot be read or parsed.
func (r *runner) readEvent(fs afero.Fs, ev *Event, eventPath string) error {
	event, err := fs.Open(eventPath)
	if err != nil {
		return fmt.Errorf("read GITHUB_EVENT_PATH: %w", err)
	}
	if err := json.NewDecoder(event).Decode(&ev); err != nil {
		return fmt.Errorf("unmarshal GITHUB_EVENT_PATH: %w", err)
	}
	return nil
}
