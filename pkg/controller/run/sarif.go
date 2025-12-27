package run

import (
	"encoding/json"
	"fmt"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/sarif"
)

const (
	ruleUnpinnedAction = "unpinned-action"
	ruleParseError     = "parse-error"
)

// outputSARIF outputs findings in SARIF format to stdout.
func (c *Controller) outputSARIF() error {
	log := sarif.Log{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarif.Run{
			{
				Tool: sarif.Tool{
					Driver: sarif.Driver{
						Name:           "pinact",
						InformationURI: "https://github.com/suzuki-shunsuke/pinact",
						Rules: []sarif.Rule{
							{
								ID: ruleUnpinnedAction,
								ShortDescription: sarif.Message{
									Text: "GitHub Action is not pinned to a commit SHA",
								},
							},
							{
								ID: ruleParseError,
								ShortDescription: sarif.Message{
									Text: "Failed to parse or process action",
								},
							},
						},
					},
				},
				Results: c.buildSARIFResults(),
			},
		},
	}

	encoder := json.NewEncoder(c.param.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(log); err != nil {
		return fmt.Errorf("encode SARIF: %w", err)
	}
	return nil
}

func (c *Controller) buildSARIFResults() []sarif.Result {
	results := make([]sarif.Result, 0, len(c.param.Findings))
	for _, f := range c.param.Findings {
		ruleID := ruleUnpinnedAction
		level := "warning"
		var msg string
		if f.Message != "" {
			// Parse error
			ruleID = ruleParseError
			level = "error"
			msg = f.Message
		} else {
			// Unpinned action
			msg = "Action should be pinned: " + f.OldLine
			if f.NewLine != "" {
				msg = "Action should be pinned: " + f.OldLine + " -> " + f.NewLine
			}
		}

		results = append(results, sarif.Result{
			RuleID:  ruleID,
			Level:   level,
			Message: sarif.Message{Text: msg},
			Locations: []sarif.Location{
				{
					PhysicalLocation: sarif.PhysicalLocation{
						ArtifactLocation: sarif.ArtifactLocation{
							URI: f.File,
						},
						Region: sarif.Region{
							StartLine: f.Line,
						},
					},
				},
			},
		})
	}
	return results
}
