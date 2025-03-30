package config_test

import (
	"testing"

	"github.com/suzuki-shunsuke/pinact/v2/pkg/config"
)

func TestIgnoreAction_Match(t *testing.T) {
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
			name: "match by name and ref (v3)",
			ignoreAction: &config.IgnoreAction{
				Name: "actions/checkout",
				Ref:  "main",
			},
			actionName:    "actions/checkout",
			actionRef:     "main",
			expected:      true,
			configVersion: 3,
		},
		{
			name: "not match (v3)",
			ignoreAction: &config.IgnoreAction{
				Name: "actions/checkout",
				Ref:  "main",
			},
			actionName:    "actions/checkout",
			actionRef:     "main-malicous",
			expected:      false,
			configVersion: 3,
		},
		{
			name: "not match name (v3)",
			ignoreAction: &config.IgnoreAction{
				Name: "actions/",
				Ref:  "main",
			},
			actionName:    "actions/checkout",
			actionRef:     "main",
			expected:      false,
			configVersion: 3,
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
