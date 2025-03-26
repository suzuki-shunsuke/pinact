package config_test

import (
	"testing"

	"github.com/suzuki-shunsuke/pinact/pkg/config"
)

func TestIgnoreAction_Match(t *testing.T) { //nolint:funlen
	t.Parallel()
	data := []struct {
		name          string
		ignoreAction  *config.IgnoreAction
		actionName    string
		actionRef     string
		configVersion int
		expected      bool
	}{
		{
			name: "match by name only",
			ignoreAction: &config.IgnoreAction{
				Name: "actions/checkout",
			},
			actionName:    "actions/checkout",
			actionRef:     "main",
			expected:      true,
			configVersion: 2,
		},
		{
			name: "match by name and ref",
			ignoreAction: &config.IgnoreAction{
				Name: "actions/checkout",
				Ref:  "main",
			},
			actionName:    "actions/checkout",
			actionRef:     "main",
			expected:      true,
			configVersion: 2,
		},
		{
			name: "match by name but not by ref",
			ignoreAction: &config.IgnoreAction{
				Name: "actions/checkout",
				Ref:  "main",
			},
			actionName:    "actions/checkout",
			actionRef:     "develop",
			expected:      false,
			configVersion: 2,
		},
		{
			name: "match by regex name",
			ignoreAction: &config.IgnoreAction{
				Name: "^actions/.*",
			},
			actionName:    "actions/checkout",
			actionRef:     "main",
			expected:      true,
			configVersion: 2,
		},
		{
			name: "match by regex ref",
			ignoreAction: &config.IgnoreAction{
				Name: "actions/checkout",
				Ref:  "^v\\d+\\.\\d+\\.\\d+$",
			},
			actionName:    "actions/checkout",
			actionRef:     "v3.5.2",
			expected:      true,
			configVersion: 2,
		},
		{
			name: "match by regex ref but not match",
			ignoreAction: &config.IgnoreAction{
				Name: "actions/checkout",
				Ref:  "^v\\d+\\.\\d+\\.\\d+$",
			},
			actionName:    "actions/checkout",
			actionRef:     "main",
			expected:      false,
			configVersion: 2,
		},
		{
			name: "not match by name",
			ignoreAction: &config.IgnoreAction{
				Name: "actions/checkout",
			},
			actionName:    "actions/setup-go",
			actionRef:     "main",
			expected:      false,
			configVersion: 2,
		},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			if err := d.ignoreAction.Init(d.configVersion); err != nil {
				t.Fatalf("failed to initialize ignore action: %v", err)
			}
			got, err := d.ignoreAction.Match(d.actionName, d.actionRef, d.configVersion)
			if err != nil {
				t.Fatalf("failed to match: %v", err)
			}
			if got != d.expected {
				t.Fatalf("wanted %v, got %v", d.expected, got)
			}
		})
	}
}
