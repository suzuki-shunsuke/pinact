package di

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/afero"
)

// Event represents a GitHub Actions event payload.
type Event struct {
	PullRequest *PullRequest `json:"pull_request"`
	Issue       *Issue       `json:"issue"`
	Repository  *Repository  `json:"repository"`
}

// RepoName extracts the repository name from the GitHub event.
func (e *Event) RepoName() string {
	if e != nil && e.Repository != nil {
		return e.Repository.Name
	}
	return ""
}

// PRNumber extracts the pull request or issue number from the GitHub event.
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
func (e *Event) SHA() string {
	if e == nil {
		return ""
	}
	if e.PullRequest != nil && e.PullRequest.Head != nil {
		return e.PullRequest.Head.SHA
	}
	return ""
}

// Issue represents a GitHub issue in the event payload.
type Issue struct {
	Number int `json:"number"`
}

// PullRequest represents a GitHub pull request in the event payload.
type PullRequest struct {
	Number int   `json:"number"`
	Head   *Head `json:"head"`
}

// Repository represents a GitHub repository in the event payload.
type Repository struct {
	Owner *Owner `json:"owner"`
	Name  string `json:"name"`
}

// Owner represents a GitHub repository owner in the event payload.
type Owner struct {
	Login string `json:"login"`
}

// Head represents the head branch of a pull request.
type Head struct {
	SHA string `json:"sha"`
}

func readEvent(fs afero.Fs, ev *Event, eventPath string) error {
	event, err := fs.Open(eventPath)
	if err != nil {
		return fmt.Errorf("read GITHUB_EVENT_PATH: %w", err)
	}
	if err := json.NewDecoder(event).Decode(&ev); err != nil {
		return fmt.Errorf("unmarshal GITHUB_EVENT_PATH: %w", err)
	}
	return nil
}
