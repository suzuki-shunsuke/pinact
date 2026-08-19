package updaterepos //nolint:testpackage // tests unexported runner helpers

import "testing"

func TestRunnerParam(t *testing.T) {
	t.Parallel()
	runner := &runner{
		organizations: []string{"acme"},
		repositories:  []string{"acme/service"},
		includeRepos:  []string{"acme/.*"},
		branchToTags:  []string{"^main$"},
		concurrency:   2,
		format:        "json",
	}
	param, err := runner.param()
	if err != nil {
		t.Fatal(err)
	}
	if len(param.Organizations) != 1 || len(param.Repositories) != 1 || param.Concurrency != 2 || param.Format != "json" {
		t.Fatalf("unexpected param: %#v", param)
	}
	if !param.IncludeRepositories[0].MatchString("acme/service") || !param.BranchToTags[0].MatchString("main") {
		t.Fatalf("regular expressions were not compiled: %#v", param)
	}
}

func TestRunnerParamRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	if _, err := (&runner{concurrency: 1}).param(); err == nil {
		t.Fatal("param accepted no organization or repository")
	}
	if _, err := (&runner{organizations: []string{"acme"}, concurrency: 0}).param(); err == nil {
		t.Fatal("param accepted invalid concurrency")
	}
	if _, err := (&runner{organizations: []string{"acme"}, concurrency: 1, includeRepos: []string{"["}}).param(); err == nil {
		t.Fatal("param accepted invalid regular expression")
	}
}
