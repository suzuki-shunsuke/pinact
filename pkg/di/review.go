package di

import (
	"fmt"
	"strings"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/run"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
)

func setReview(fs afero.Fs, review *run.Review, flags *Flags) error {
	if review.RepoName == "" {
		repo := flags.GitHubRepository
		_, repoName, ok := strings.Cut(repo, "/")
		if !ok {
			return fmt.Errorf("GITHUB_REPOSITORY is not set or invalid: %s", repo)
		}
		if repoName == "" {
			return fmt.Errorf("GITHUB_REPOSITORY is invalid: %s", repo)
		}
		review.RepoName = repoName
	}
	if flags.GitHubEventPath == "" {
		return nil
	}
	var ev *Event
	if review.PullRequest == 0 {
		ev = &Event{}
		if err := readEvent(fs, ev, flags.GitHubEventPath); err != nil {
			return err
		}
		review.PullRequest = ev.PRNumber()
	}
	if review.SHA != "" {
		return nil
	}
	if ev == nil {
		ev = &Event{}
		if err := readEvent(fs, ev, flags.GitHubEventPath); err != nil {
			return err
		}
	}
	review.SHA = ev.SHA()
	return nil
}

func setupReview(fs afero.Fs, logger *slogutil.Logger, flags *Flags) *run.Review {
	if !flags.Review {
		return nil
	}
	review := &run.Review{
		RepoOwner:   flags.RepoOwner,
		RepoName:    flags.RepoName,
		PullRequest: flags.PR,
		SHA:         flags.SHA,
	}
	if flags.IsGitHubActions {
		if err := setReview(fs, review, flags); err != nil {
			slogerr.WithError(logger.Logger, err).Error("set review information")
		}
	}
	if !review.Valid() {
		logger.Warn("skip creating reviews because the review information is invalid")
		return nil
	}
	return review
}
