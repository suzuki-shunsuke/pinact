package config_test

import (
	"testing"

	"github.com/suzuki-shunsuke/pinact/v4/pkg/config"
)

func TestIgnoreAction_Match(t *testing.T) {
	t.Parallel()
	data := []struct {
		name         string
		ignoreAction *config.IgnoreAction
		actionName   string
		actionRef    string
		expected     bool
	}{
		{
			name: "match by name and ref",
			ignoreAction: &config.IgnoreAction{
				Name: "actions/checkout",
				Ref:  "main",
			},
			actionName: "actions/checkout",
			actionRef:  "main",
			expected:   true,
		},
		{
			name: "not match",
			ignoreAction: &config.IgnoreAction{
				Name: "actions/checkout",
				Ref:  "main",
			},
			actionName: "actions/checkout",
			actionRef:  "main-malicious",
			expected:   false,
		},
		{
			name: "not match name",
			ignoreAction: &config.IgnoreAction{
				Name: "actions/",
				Ref:  "main",
			},
			actionName: "actions/checkout",
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
			if got := d.ignoreAction.Match(d.actionName, d.actionRef); got != d.expected {
				t.Fatalf("wanted %v, got %v", d.expected, got)
			}
		})
	}
}
