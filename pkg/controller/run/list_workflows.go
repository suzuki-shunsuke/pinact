package run

import (
	"fmt"
	"path/filepath"

	"github.com/suzuki-shunsuke/slog-error/slogerr"
)

// defaultWorkflowPatterns is the set of glob patterns used to discover GitHub
// Actions workflow and composite action files when neither command line args
// nor a config file provide explicit target patterns. The patterns use `/` as
// the path separator and are intended for both filepath.Glob (disk discovery)
// and path.Match (filtering an already-known set of paths, e.g. from
// --diff-file).
//
//nolint:gochecknoglobals // shared read-only pattern list used for both disk discovery (filepath.Glob) and diff filtering (path.Match)
var defaultWorkflowPatterns = []string{
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

// listWorkflows discovers GitHub Actions workflow and composite action files.
// It searches for YAML files in standard locations including .github/workflows
// and action.yaml files in various directory structures.
//
// Returns a slice of discovered file paths or an error if globbing fails.
func listWorkflows() ([]string, error) {
	files := []string{}
	for _, pattern := range defaultWorkflowPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("look for workflow or composite action files using glob: %w", slogerr.With(err, "pattern", pattern))
		}
		files = append(files, matches...)
	}
	return files, nil
}
