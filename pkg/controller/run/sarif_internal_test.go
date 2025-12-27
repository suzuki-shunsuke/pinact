package run

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/sarif"
)

func TestController_outputSARIF(t *testing.T) { //nolint:funlen
	t.Parallel()
	tests := []struct {
		name     string
		findings []Finding
		wantErr  bool
	}{
		{
			name:     "empty findings",
			findings: nil,
			wantErr:  false,
		},
		{
			name: "single unpinned action",
			findings: []Finding{
				{
					File:    ".github/workflows/test.yml",
					Line:    10,
					OldLine: "  - uses: actions/checkout@v4",
					NewLine: "  - uses: actions/checkout@abc123 # v4",
				},
			},
			wantErr: false,
		},
		{
			name: "multiple findings",
			findings: []Finding{
				{
					File:    ".github/workflows/ci.yml",
					Line:    5,
					OldLine: "  - uses: actions/checkout@v4",
					NewLine: "  - uses: actions/checkout@abc123 # v4",
				},
				{
					File:    ".github/workflows/ci.yml",
					Line:    10,
					OldLine: "  - uses: actions/setup-node@v4",
					NewLine: "  - uses: actions/setup-node@def456 # v4",
				},
			},
			wantErr: false,
		},
		{
			name: "parse error finding",
			findings: []Finding{
				{
					File:    ".github/workflows/test.yml",
					Line:    15,
					OldLine: "  - uses: invalid-action",
					Message: "failed to handle a line: invalid action format",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			c := &Controller{
				param: &ParamRun{
					Stdout:   buf,
					Stderr:   &bytes.Buffer{},
					Findings: tt.findings,
				},
				logger: NewLogger(&bytes.Buffer{}),
			}

			err := c.outputSARIF()
			if (err != nil) != tt.wantErr {
				t.Errorf("outputSARIF() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify valid JSON output
			var log sarif.Log
			if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
				t.Errorf("outputSARIF() produced invalid JSON: %v", err)
				return
			}

			// Verify SARIF structure
			if log.Schema != "https://json.schemastore.org/sarif-2.1.0.json" {
				t.Errorf("outputSARIF() schema = %v, want %v", log.Schema, "https://json.schemastore.org/sarif-2.1.0.json")
			}
			if log.Version != "2.1.0" {
				t.Errorf("outputSARIF() version = %v, want %v", log.Version, "2.1.0")
			}
			if len(log.Runs) != 1 {
				t.Errorf("outputSARIF() runs count = %v, want 1", len(log.Runs))
				return
			}
			if log.Runs[0].Tool.Driver.Name != "pinact" {
				t.Errorf("outputSARIF() tool name = %v, want pinact", log.Runs[0].Tool.Driver.Name)
			}
			if len(log.Runs[0].Results) != len(tt.findings) {
				t.Errorf("outputSARIF() results count = %v, want %v", len(log.Runs[0].Results), len(tt.findings))
			}
		})
	}
}

func TestController_buildSARIFResults(t *testing.T) { //nolint:funlen,cyclop
	t.Parallel()
	tests := []struct {
		name      string
		findings  []Finding
		wantCount int
		validate  func(t *testing.T, results []sarif.Result)
	}{
		{
			name:      "empty findings",
			findings:  nil,
			wantCount: 0,
			validate:  nil,
		},
		{
			name: "unpinned action with new line",
			findings: []Finding{
				{
					File:    "test.yml",
					Line:    10,
					OldLine: "  - uses: actions/checkout@v4",
					NewLine: "  - uses: actions/checkout@abc123 # v4",
				},
			},
			wantCount: 1,
			validate: func(t *testing.T, results []sarif.Result) {
				t.Helper()
				r := results[0]
				if r.RuleID != ruleUnpinnedAction {
					t.Errorf("RuleID = %v, want %v", r.RuleID, ruleUnpinnedAction)
				}
				if r.Level != levelError {
					t.Errorf("Level = %v, want %v", r.Level, levelError)
				}
				if r.Locations[0].PhysicalLocation.ArtifactLocation.URI != "test.yml" {
					t.Errorf("URI = %v, want test.yml", r.Locations[0].PhysicalLocation.ArtifactLocation.URI)
				}
				if r.Locations[0].PhysicalLocation.Region.StartLine != 10 {
					t.Errorf("StartLine = %v, want 10", r.Locations[0].PhysicalLocation.Region.StartLine)
				}
			},
		},
		{
			name: "unpinned action without new line",
			findings: []Finding{
				{
					File:    "test.yml",
					Line:    5,
					OldLine: "  - uses: actions/checkout@v4",
					NewLine: "",
				},
			},
			wantCount: 1,
			validate: func(t *testing.T, results []sarif.Result) {
				t.Helper()
				r := results[0]
				if r.RuleID != ruleUnpinnedAction {
					t.Errorf("RuleID = %v, want %v", r.RuleID, ruleUnpinnedAction)
				}
				expected := "Action should be pinned:   - uses: actions/checkout@v4"
				if r.Message.Text != expected {
					t.Errorf("Message = %v, want %v", r.Message.Text, expected)
				}
			},
		},
		{
			name: "parse error",
			findings: []Finding{
				{
					File:    "workflow.yml",
					Line:    20,
					OldLine: "  - uses: invalid",
					Message: "failed to parse action",
				},
			},
			wantCount: 1,
			validate: func(t *testing.T, results []sarif.Result) {
				t.Helper()
				r := results[0]
				if r.RuleID != ruleParseError {
					t.Errorf("RuleID = %v, want %v", r.RuleID, ruleParseError)
				}
				if r.Level != levelError {
					t.Errorf("Level = %v, want %v", r.Level, levelError)
				}
				if r.Message.Text != "failed to parse action" {
					t.Errorf("Message = %v, want 'failed to parse action'", r.Message.Text)
				}
			},
		},
		{
			name: "mixed findings",
			findings: []Finding{
				{
					File:    "ci.yml",
					Line:    5,
					OldLine: "  - uses: actions/checkout@v4",
					NewLine: "  - uses: actions/checkout@abc # v4",
				},
				{
					File:    "ci.yml",
					Line:    10,
					OldLine: "  - uses: invalid",
					Message: "parse error",
				},
			},
			wantCount: 2,
			validate: func(t *testing.T, results []sarif.Result) {
				t.Helper()
				if results[0].RuleID != ruleUnpinnedAction {
					t.Errorf("First result RuleID = %v, want %v", results[0].RuleID, ruleUnpinnedAction)
				}
				if results[1].RuleID != ruleParseError {
					t.Errorf("Second result RuleID = %v, want %v", results[1].RuleID, ruleParseError)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := &Controller{
				param: &ParamRun{
					Findings: tt.findings,
				},
			}

			results := c.buildSARIFResults()

			if len(results) != tt.wantCount {
				t.Errorf("buildSARIFResults() count = %v, want %v", len(results), tt.wantCount)
				return
			}

			if tt.validate != nil {
				tt.validate(t, results)
			}
		})
	}
}
