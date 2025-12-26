package run

import (
	"fmt"
	"path/filepath"

	"github.com/suzuki-shunsuke/slog-error/slogerr"
)

// ListWorkflows discovers GitHub Actions workflow and composite action files.
// It searches for YAML files in standard locations including .github/workflows
// and action.yaml files in various directory structures.
//
// Returns a slice of discovered file paths or an error if globbing fails.
func ListWorkflows() ([]string, error) {
	patterns := []string{
		".github/workflows/*.yml",
		".github/workflows/*.yaml",
		"action.yml",
		"action.yaml",
		"*/action.yml",
		"*/action.yaml",
		"*/*/action.yml",
		"*/*/action.yaml",
		"*/*/*/action.yml",
		"*/*/*/action.yaml",
	}
	files := []string{}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("look for workflow or composite action files using glob: %w", slogerr.With(err, "pattern", pattern))
		}
		files = append(files, matches...)
	}
	return files, nil
}
