package run

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
)

func strP(s string) *string {
	return &s
}

func Test_parseAction(t *testing.T) { //nolint:funlen
	t.Parallel()
	data := []struct {
		name string
		line string
		exp  *Action
	}{
		{
			name: "unrelated",
			line: "unrelated",
		},
		{
			name: "checkout v3",
			line: "  - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3",
			exp: &Action{
				Uses:                    "  - uses: ",
				Name:                    "actions/checkout",
				Version:                 "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionCommentSeparator: " # ",
				VersionComment:          "v3",
			},
		},
		{
			name: "checkout v2",
			line: "  uses: actions/checkout@v2",
			exp: &Action{
				Uses:    "  uses: ",
				Name:    "actions/checkout",
				Version: "v2",
			},
		},
		{
			name: "checkout v3 (single quote)",
			line: `  - "uses": 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # v3`,
			exp: &Action{
				Uses:                    `  - "uses": `,
				Name:                    "actions/checkout",
				Version:                 "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionCommentSeparator: " # ",
				VersionComment:          "v3",
				Quote:                   "'",
			},
		},
		{
			name: "checkout v3 (double quote)",
			line: `  - 'uses': "actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab" # v3`,
			exp: &Action{
				Uses:                    `  - 'uses': `,
				Name:                    "actions/checkout",
				Version:                 "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionCommentSeparator: " # ",
				VersionComment:          "v3",
				Quote:                   `"`,
			},
		},
		{
			name: "checkout v2 (single quote)",
			line: `  "uses": 'actions/checkout@v2'`,
			exp: &Action{
				Uses:           `  "uses": `,
				Name:           "actions/checkout",
				Version:        "v2",
				VersionComment: "",
				Quote:          `'`,
			},
		},
		{
			name: "checkout v2 (double quote)",
			line: `  'uses': "actions/checkout@v2"`,
			exp: &Action{
				Uses:           `  'uses': `,
				Name:           "actions/checkout",
				Version:        "v2",
				VersionComment: "",
				Quote:          `"`,
			},
		},
		{
			name: "tag=",
			line: `      - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # tag=v3`,
			exp: &Action{
				Uses:                    `      - uses: `,
				Name:                    "actions/checkout",
				Version:                 "83b7061638ee4956cf7545a6f7efe594e5ad0247",
				VersionCommentSeparator: " # tag=",
				VersionComment:          "v3",
				Quote:                   "",
			},
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			act := parseAction(d.line)
			if diff := cmp.Diff(d.exp, act); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestController_parseLine(t *testing.T) { //nolint:funlen
	t.Parallel()
	data := []struct {
		name  string
		line  string
		exp   string
		isErr bool
	}{
		{
			name: "unrelated",
			line: "unrelated",
		},
		{
			name: "checkout v3",
			line: "  - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3",
			exp:  "  - uses: actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab # v3.5.2",
		},
		{
			name: "checkout v2",
			line: "  uses: actions/checkout@v2",
			exp:  "  uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.7.0",
		},
		{
			name: "single quote",
			line: `  - "uses": 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # v3`,
			exp:  `  - "uses": 'actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab' # v3.5.2`,
		},
		{
			name: "double quote",
			line: `  - 'uses': "actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab" # v3`,
			exp:  `  - 'uses': "actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab" # v3.5.2`,
		},
		{
			name: "checkout v2 (single quote)",
			line: `  "uses": 'actions/checkout@v2'`,
			exp:  `  "uses": 'actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5' # v2.7.0`,
		},
	}
	logE := logrus.NewEntry(logrus.New())
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			fs := afero.NewMemMapFs()
			ctrl := New(&RepositoriesServiceImpl{
				Tags: map[string]*ListTagsResult{
					"actions/checkout/0": {
						Tags: []*github.RepositoryTag{
							{
								Name: strP("v3"),
								Commit: &github.Commit{
									SHA: strP("8e5e7e5ab8b370d6c329ec480221332ada57f0ab"),
								},
							},
							{
								Name: strP("v3.5.2"),
								Commit: &github.Commit{
									SHA: strP("8e5e7e5ab8b370d6c329ec480221332ada57f0ab"),
								},
							},
							{
								Name: strP("v2"),
								Commit: &github.Commit{
									SHA: strP("ee0669bd1cc54295c223e0bb666b733df41de1c5"),
								},
							},
							{
								Name: strP("v2.7.0"),
								Commit: &github.Commit{
									SHA: strP("ee0669bd1cc54295c223e0bb666b733df41de1c5"),
								},
							},
						},
						Response: &github.Response{},
					},
				},
				Commits: map[string]*GetCommitSHA1Result{
					"actions/checkout/v3": {
						SHA: "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
					},
					"actions/checkout/v2": {
						SHA: "ee0669bd1cc54295c223e0bb666b733df41de1c5",
					},
				},
			}, nil, fs, config.NewFinder(fs), config.NewReader(fs), &ParamRun{})
			line, err := ctrl.parseLine(t.Context(), logE, d.line)
			if err != nil {
				if d.isErr {
					return
				}
				t.Fatal(err)
			}
			if line != d.exp {
				t.Fatalf(`wanted %s, got %s`, d.exp, line)
			}
		})
	}
}

func Test_patchLine(t *testing.T) {
	t.Parallel()
	data := []struct {
		name    string
		tag     string
		version string
		action  *Action
		exp     string
	}{
		{
			name: "checkout v3",
			exp:  "  - uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v3.5.2",
			action: &Action{
				Uses:                    "  - uses: ",
				Name:                    "actions/checkout",
				Version:                 "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionCommentSeparator: " # ",
				VersionComment:          "v3",
			},
			version: "ee0669bd1cc54295c223e0bb666b733df41de1c5",
			tag:     "v3.5.2",
		},
		{
			name: "checkout v2",
			exp:  "  uses: actions/checkout@ee0669bd1cc54295c223e0bb666b733df41de1c5 # v2.17.0",
			action: &Action{
				Uses:    "  uses: ",
				Name:    "actions/checkout",
				Version: "v2",
			},
			version: "ee0669bd1cc54295c223e0bb666b733df41de1c5",
			tag:     "v2.17.0",
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			line := patchLine(d.action, d.version, d.tag)
			if line != d.exp {
				t.Fatalf(`wanted %s, got %s`, d.exp, line)
			}
		})
	}
}
