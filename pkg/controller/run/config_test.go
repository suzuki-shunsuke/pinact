package run_test

import (
	"testing"

	"github.com/suzuki-shunsuke/pinact/pkg/controller/run"
)

func TestIgnoreAction_Match(t *testing.T) { //nolint:funlen
	t.Parallel()
	data := []struct {
		name       string
		configName string
		configRef  string
		actionName string
		actionRef  string
		expected   bool
	}{
		{
			name:       "match by name only",
			configName: "actions/checkout",
			configRef:  "",
			actionName: "actions/checkout",
			actionRef:  "main",
			expected:   true,
		},
		{
			name:       "match by name and ref",
			configName: "actions/checkout",
			configRef:  "main",
			actionName: "actions/checkout",
			actionRef:  "main",
			expected:   true,
		},
		{
			name:       "match by name but not by ref",
			configName: "actions/checkout",
			configRef:  "main",
			actionName: "actions/checkout",
			actionRef:  "develop",
			expected:   false,
		},
		{
			name:       "match by regex name",
			configName: "^actions/.*",
			configRef:  "",
			actionName: "actions/checkout",
			actionRef:  "main",
			expected:   true,
		},
		{
			name:       "match by regex ref",
			configName: "actions/checkout",
			configRef:  "^v\\d+\\.\\d+\\.\\d+$",
			actionName: "actions/checkout",
			actionRef:  "v3.5.2",
			expected:   true,
		},
		{
			name:       "match by regex ref but not match",
			configName: "actions/checkout",
			configRef:  "^v\\d+\\.\\d+\\.\\d+$",
			actionName: "actions/checkout",
			actionRef:  "main",
			expected:   false,
		},
		{
			name:       "not match by name",
			configName: "actions/checkout",
			configRef:  "",
			actionName: "actions/setup-go",
			actionRef:  "main",
			expected:   false,
		},
	}

	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			config, err := run.NewIgnoreAction(d.configName, d.configRef)
			if err != nil {
				t.Fatalf("failed to create ignore action: %v", err)
			}

			got := config.Match(d.actionName, d.actionRef)
			if got != d.expected {
				t.Fatalf("wanted %v, got %v", d.expected, got)
			}
		})
	}
}
