package run

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	logger := NewLogger(buf)

	if logger == nil {
		t.Fatal("NewLogger() returned nil")
	}
	if logger.stderr != buf {
		t.Error("NewLogger() stderr not set correctly")
	}
	if logger.red == nil {
		t.Error("NewLogger() red function is nil")
	}
	if logger.green == nil {
		t.Error("NewLogger() green function is nil")
	}
}

func TestLogger_Output(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name           string
		level          string
		message        string
		line           *Line
		newLine        string
		wantContains   []string
		wantNotContain []string
	}{
		{
			name:    "info level without new line",
			level:   "info",
			message: "action isn't pinned",
			line: &Line{
				File:   "test.yml",
				Number: 10,
				Line:   "    uses: actions/checkout@v3",
			},
			newLine: "",
			wantContains: []string{
				"INFO",
				"action isn't pinned",
				"test.yml:10",
				"    uses: actions/checkout@v3",
			},
			wantNotContain: []string{
				"ERROR",
				"+ ",
			},
		},
		{
			name:    "error level without new line",
			level:   "error",
			message: "failed to handle a line",
			line: &Line{
				File:   "workflow.yml",
				Number: 25,
				Line:   "  - uses: custom/action@main",
			},
			newLine: "",
			wantContains: []string{
				"ERROR",
				"failed to handle a line",
				"workflow.yml:25",
			},
		},
		{
			name:    "info level with diff",
			level:   "info",
			message: "action isn't pinned",
			line: &Line{
				File:   "test.yml",
				Number: 10,
				Line:   "  - uses: actions/checkout@v3",
			},
			newLine: "  - uses: actions/checkout@abc123 # v3",
			wantContains: []string{
				"INFO",
				"action isn't pinned",
				"test.yml:10",
				"- ",                           // old line prefix
				"+ ",                           // new line prefix
				"actions/checkout@v3",          // old version
				"actions/checkout@abc123 # v3", // new version
			},
		},
		{
			name:    "error level with diff",
			level:   "error",
			message: "action isn't pinned",
			line: &Line{
				File:   "ci.yml",
				Number: 5,
				Line:   "  uses: owner/repo@v1",
			},
			newLine: "  uses: owner/repo@def456 # v1.0.0",
			wantContains: []string{
				"ERROR",
				"action isn't pinned",
				"ci.yml:5",
				"- ",
				"+ ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			logger := NewLogger(buf)

			logger.Output(tt.level, tt.message, tt.line, tt.newLine)

			output := buf.String()

			for _, want := range tt.wantContains {
				if !strings.Contains(output, want) {
					t.Errorf("Output() missing expected content %q in:\n%s", want, output)
				}
			}

			for _, notWant := range tt.wantNotContain {
				if strings.Contains(output, notWant) {
					t.Errorf("Output() contains unexpected content %q in:\n%s", notWant, output)
				}
			}
		})
	}
}

func TestLogger_Output_format(t *testing.T) {
	t.Parallel()
	buf := &bytes.Buffer{}
	logger := NewLogger(buf)

	line := &Line{
		File:   "test.yml",
		Number: 42,
		Line:   "original line",
	}

	// Test without new line
	logger.Output("info", "test message", line, "")
	output := buf.String()

	// Verify format: "LEVEL message\nfile:line\noriginal\n"
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines in output without newLine, got %d: %v", len(lines), lines)
	}

	// Test with new line
	buf.Reset()
	logger.Output("info", "test message", line, "new line")
	output = buf.String()

	// Verify format: "LEVEL message\nfile:line\n- original\n+ new\n"
	lines = strings.Split(strings.TrimRight(output, "\n"), "\n")
	if len(lines) != 4 {
		t.Errorf("Expected 4 lines in output with newLine, got %d: %v", len(lines), lines)
	}
}
