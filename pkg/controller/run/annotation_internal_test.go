package run

import (
	"bytes"
	"strings"
	"testing"
)

func TestOutputGitHubActionsAnnotation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		fix       bool
		wantLevel string
		wantTitle string
	}{
		{
			name:      "fix mode: notice annotation with neutral title",
			fix:       true,
			wantLevel: "notice",
			wantTitle: "pinact",
		},
		{
			name:      "check mode: error annotation with error title",
			fix:       false,
			wantLevel: "error",
			wantTitle: "pinact error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			stderr := &bytes.Buffer{}
			c := &Controller{
				param: &ParamRun{
					IsGitHubActions: true,
					Fix:             tt.fix,
					Stderr:          stderr,
				},
				logger: NewLogger(stderr),
			}
			line := &Line{
				File:   ".github/workflows/test.yaml",
				Number: 10,
				Line:   "  - uses: actions/setup-go@v6.0.1",
			}
			newLine := "  - uses: actions/setup-go@abc123 # v6.0.2"
			c.outputGitHubActionsAnnotation(line, newLine)
			got := stderr.String()
			prefix := "::" + tt.wantLevel + " file=.github/workflows/test.yaml,line=10,title=" + tt.wantTitle + "::"
			if !strings.HasPrefix(got, prefix) {
				t.Errorf("annotation %q should start with %q", got, prefix)
			}
		})
	}
}
