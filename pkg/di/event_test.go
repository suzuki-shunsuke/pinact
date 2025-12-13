package di_test

import (
	"testing"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/di"
)

func TestEvent_RepoName(t *testing.T) {
	t.Parallel()
	data := []struct {
		name  string
		event *di.Event
		exp   string
	}{
		{name: "nil event", event: nil, exp: ""},
		{name: "nil repository", event: &di.Event{}, exp: ""},
		{name: "with repository", event: &di.Event{Repository: &di.Repository{Name: "my-repo"}}, exp: "my-repo"},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			if got := d.event.RepoName(); got != d.exp {
				t.Errorf("wanted %q, got %q", d.exp, got)
			}
		})
	}
}

func TestEvent_PRNumber(t *testing.T) {
	t.Parallel()
	data := []struct {
		name  string
		event *di.Event
		exp   int
	}{
		{name: "nil event", event: nil, exp: 0},
		{name: "empty event", event: &di.Event{}, exp: 0},
		{name: "pull request", event: &di.Event{PullRequest: &di.PullRequest{Number: 123}}, exp: 123},
		{name: "issue", event: &di.Event{Issue: &di.Issue{Number: 456}}, exp: 456},
		{
			name:  "both - pull request takes precedence",
			event: &di.Event{PullRequest: &di.PullRequest{Number: 123}, Issue: &di.Issue{Number: 456}},
			exp:   123,
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			if got := d.event.PRNumber(); got != d.exp {
				t.Errorf("wanted %d, got %d", d.exp, got)
			}
		})
	}
}

func TestEvent_SHA(t *testing.T) {
	t.Parallel()
	data := []struct {
		name  string
		event *di.Event
		exp   string
	}{
		{name: "nil event", event: nil, exp: ""},
		{name: "empty event", event: &di.Event{}, exp: ""},
		{name: "pull request without head", event: &di.Event{PullRequest: &di.PullRequest{Number: 123}}, exp: ""},
		{
			name:  "pull request with head",
			event: &di.Event{PullRequest: &di.PullRequest{Number: 123, Head: &di.Head{SHA: "abc123"}}},
			exp:   "abc123",
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			if got := d.event.SHA(); got != d.exp {
				t.Errorf("wanted %q, got %q", d.exp, got)
			}
		})
	}
}
