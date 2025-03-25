package run_test

import (
	"testing"

	"github.com/suzuki-shunsuke/pinact/pkg/controller/run"
)

func TestIgnoreAction_Match(t *testing.T) { //nolint:funlen
	t.Parallel()
	data := []struct {
		name         string
		ignoreAction *run.IgnoreAction
		actionName   string
		actionRef    string
		expected     bool
	}{
		{
			name: "match by name only",
			ignoreAction: &run.IgnoreAction{
				Name:       "actions/checkout",
				NameFormat: "fixed_string",
			},
			actionName: "actions/checkout",
			actionRef:  "main",
			expected:   true,
		},
		{
			name: "match by name and ref",
			ignoreAction: &run.IgnoreAction{
				Name:       "actions/checkout",
				NameFormat: "fixed_string",
				Ref:        "main",
				RefFormat:  "fixed_string",
			},
			actionName: "actions/checkout",
			actionRef:  "main",
			expected:   true,
		},
		{
			name: "match by name but not by ref",
			ignoreAction: &run.IgnoreAction{
				Name:       "actions/checkout",
				NameFormat: "fixed_string",
				Ref:        "main",
				RefFormat:  "fixed_string",
			},
			actionName: "actions/checkout",
			actionRef:  "develop",
			expected:   false,
		},
		{
			name: "match by regex name",
			ignoreAction: &run.IgnoreAction{
				Name:       "^actions/.*",
				NameFormat: "regexp",
			},
			actionName: "actions/checkout",
			actionRef:  "main",
			expected:   true,
		},
		{
			name: "match by regex ref",
			ignoreAction: &run.IgnoreAction{
				Name:       "actions/checkout",
				NameFormat: "fixed_string",
				Ref:        "^v\\d+\\.\\d+\\.\\d+$",
				RefFormat:  "regexp",
			},
			actionName: "actions/checkout",
			actionRef:  "v3.5.2",
			expected:   true,
		},
		{
			name: "match by regex ref but not match",
			ignoreAction: &run.IgnoreAction{
				Name:       "actions/checkout",
				NameFormat: "fixed_string",
				Ref:        "^v\\d+\\.\\d+\\.\\d+$",
				RefFormat:  "regexp",
			},
			actionName: "actions/checkout",
			actionRef:  "main",
			expected:   false,
		},
		{
			name: "not match by name",
			ignoreAction: &run.IgnoreAction{
				Name:       "actions/checkout",
				NameFormat: "fixed_string",
			},
			actionName: "actions/setup-go",
			actionRef:  "main",
			expected:   false,
		},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			if err := d.ignoreAction.Init(); err != nil {
				t.Fatalf("failed to initialize ignore action: %v", err)
			}
			got, err := d.ignoreAction.Match(d.actionName, d.actionRef)
			if err != nil {
				t.Fatalf("failed to match: %v", err)
			}
			if got != d.expected {
				t.Fatalf("wanted %v, got %v", d.expected, got)
			}
		})
	}
}
