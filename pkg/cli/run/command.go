package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/log"
	"github.com/urfave/cli/v3"
)

func New(logE *logrus.Entry) *cli.Command {
	r := &runner{
		logE: logE,
	}
	return r.Command()
}

type runner struct {
	logE *logrus.Entry
}

func (r *runner) Command() *cli.Command {
	return &cli.Command{
		Name:  "run",
		Usage: "Pin GitHub Actions versions",
		Description: `If no argument is passed, pinact searches GitHub Actions workflow files from .github/workflows.

$ pinact run

You can also pass workflow file paths as arguments.

e.g.

$ pinact run .github/actions/foo/action.yaml .github/actions/bar/action.yaml
`,
		Action: r.action,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "verify",
				Aliases: []string{"v"},
				Usage:   "Verify if pairs of commit SHA and version are correct",
			},
			&cli.BoolFlag{
				Name:  "check",
				Usage: "Exit with a non-zero status code if actions are not pinned. If this is true, files aren't updated",
			},
			&cli.BoolFlag{
				Name:    "update",
				Aliases: []string{"u"},
				Usage:   "Update actions to latest versions",
			},
			&cli.BoolFlag{
				Name:  "review",
				Usage: "Create reviews",
			},
			&cli.BoolFlag{
				Name:  "fix",
				Usage: "Fix code. By default, this is true. If -check or -diff is true, this is false by default",
			},
			&cli.BoolFlag{
				Name:  "diff",
				Usage: "Output diff. By default, this is false",
			},
			&cli.StringFlag{
				Name:    "repo-owner",
				Usage:   "GitHub repository owner",
				Sources: cli.EnvVars("GITHUB_REPOSITORY_OWNER"),
			},
			&cli.StringFlag{
				Name:  "repo-name",
				Usage: "GitHub repository name",
			},
			&cli.StringFlag{
				Name:  "sha",
				Usage: "Commit SHA to be reviewed",
			},
			&cli.IntFlag{
				Name:  "pr",
				Usage: "GitHub pull request number",
			},
		},
	}
}

type Event struct {
	PullRequest *PullRequest `json:"pull_request"`
	Issue       *Issue       `json:"issue"`
	Repository  *Repository  `json:"repository"`
}

func (e *Event) RepoName() string {
	if e != nil && e.Repository != nil {
		return e.Repository.Name
	}
	return ""
}

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

func (r *runner) action(ctx context.Context, c *cli.Command) error {
	log.SetLevel(c.String("log-level"), r.logE)
	pwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get the current directory: %w", err)
	}

	isGitHubActions := os.Getenv("GITHUB_ACTIONS") == "true"

	gh := github.New(ctx, r.logE)
	fs := afero.NewOsFs()
	var review *run.Review
	if c.Bool("review") {
		review = &run.Review{
			RepoOwner:   c.String("repo-owner"),
			RepoName:    c.String("repo-name"),
			PullRequest: c.Int("pr"),
			SHA:         c.String("sha"),
		}
		if isGitHubActions {
			if err := r.setReview(fs, review); err != nil {
				logerr.WithError(r.logE, err).Error("set review information")
			}
		}
		if !review.Valid() {
			r.logE.Warn("skip creating reviews because the review information is invalid")
			review = nil
		}
	}
	param := &run.ParamRun{
		WorkflowFilePaths: c.Args().Slice(),
		ConfigFilePath:    c.String("config"),
		PWD:               pwd,
		IsVerify:          c.Bool("verify"),
		Check:             c.Bool("check"),
		Update:            c.Bool("update"),
		Diff:              c.Bool("diff"),
		Fix:               true,
		IsGitHubActions:   isGitHubActions,
		Stderr:            os.Stderr,
		Review:            review,
	}
	if c.IsSet("fix") {
		param.Fix = c.Bool("fix")
	} else if param.Check || param.Diff {
		param.Fix = false
	}
	ctrl := run.New(&run.RepositoriesServiceImpl{
		Tags:                map[string]*run.ListTagsResult{},
		Releases:            map[string]*run.ListReleasesResult{},
		Commits:             map[string]*run.GetCommitSHA1Result{},
		RepositoriesService: gh.Repositories,
	}, gh.PullRequests, fs, config.NewFinder(fs), config.NewReader(fs), param)
	return ctrl.Run(ctx, r.logE) //nolint:wrapcheck
}

func (r *runner) setReview(fs afero.Fs, review *run.Review) error {
	if review.RepoName == "" {
		review.RepoName = os.Getenv("GITHUB_REPOSITORY_OWNER")
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
		review.SHA = ev.SHA()
	}
	return nil
}

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
