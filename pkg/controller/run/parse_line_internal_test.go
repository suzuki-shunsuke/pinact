package run

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/pkg/github"
	"github.com/suzuki-shunsuke/pinact/pkg/util"
)

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
				Uses:                "  - uses: ",
				Name:                "actions/checkout",
				Version:             "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionTagSeparator: " # ",
				Tag:                 "v3",
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
				Uses:                `  - "uses": `,
				Name:                "actions/checkout",
				Version:             "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionTagSeparator: " # ",
				Tag:                 "v3",
				Quote:               "'",
			},
		},
		{
			name: "checkout v3 (double quote)",
			line: `  - 'uses': "actions/checkout@8e5e7e5ab8b370d6c329ec480221332ada57f0ab" # v3`,
			exp: &Action{
				Uses:                `  - 'uses': `,
				Name:                "actions/checkout",
				Version:             "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionTagSeparator: " # ",
				Tag:                 "v3",
				Quote:               `"`,
			},
		},
		{
			name: "checkout v2 (single quote)",
			line: `  "uses": 'actions/checkout@v2'`,
			exp: &Action{
				Uses:    `  "uses": `,
				Name:    "actions/checkout",
				Version: "v2",
				Tag:     "",
				Quote:   `'`,
			},
		},
		{
			name: "checkout v2 (double quote)",
			line: `  'uses': "actions/checkout@v2"`,
			exp: &Action{
				Uses:    `  'uses': `,
				Name:    "actions/checkout",
				Version: "v2",
				Tag:     "",
				Quote:   `"`,
			},
		},
		{
			name: "tag=",
			line: `      - uses: actions/checkout@83b7061638ee4956cf7545a6f7efe594e5ad0247 # tag=v3`,
			exp: &Action{
				Uses:                `      - uses: `,
				Name:                "actions/checkout",
				Version:             "83b7061638ee4956cf7545a6f7efe594e5ad0247",
				VersionTagSeparator: " # tag=",
				Tag:                 "v3",
				Quote:               "",
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
			exp:  "unrelated",
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
	ctx := context.Background()
	logE := logrus.NewEntry(logrus.New())
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			ctrl := NewController(&RepositoriesServiceImpl{
				tags: map[string]*ListTagsResult{
					"actions/checkout/0": {
						Tags: []*github.RepositoryTag{
							{
								Name: util.StrP("v3"),
								Commit: &github.Commit{
									SHA: util.StrP("8e5e7e5ab8b370d6c329ec480221332ada57f0ab"),
								},
							},
							{
								Name: util.StrP("v3.5.2"),
								Commit: &github.Commit{
									SHA: util.StrP("8e5e7e5ab8b370d6c329ec480221332ada57f0ab"),
								},
							},
							{
								Name: util.StrP("v2"),
								Commit: &github.Commit{
									SHA: util.StrP("ee0669bd1cc54295c223e0bb666b733df41de1c5"),
								},
							},
							{
								Name: util.StrP("v2.7.0"),
								Commit: &github.Commit{
									SHA: util.StrP("ee0669bd1cc54295c223e0bb666b733df41de1c5"),
								},
							},
						},
						Response: &github.Response{},
					},
				},
				commits: map[string]*GetCommitSHA1Result{
					"actions/checkout/v3": {
						SHA: "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
					},
					"actions/checkout/v2": {
						SHA: "ee0669bd1cc54295c223e0bb666b733df41de1c5",
					},
				},
			}, afero.NewMemMapFs())
			line, err := ctrl.parseLine(ctx, logE, d.line, &Config{})
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
				Uses:                "  - uses: ",
				Name:                "actions/checkout",
				Version:             "8e5e7e5ab8b370d6c329ec480221332ada57f0ab",
				VersionTagSeparator: " # ",
				Tag:                 "v3",
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
