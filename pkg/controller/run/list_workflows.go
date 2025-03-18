package run

import (
	"fmt"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
)

func listWorkflows() ([]string, error) {
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
			return nil, fmt.Errorf("look for workflow or composite action files using glob: %w", logerr.WithFields(err, logrus.Fields{
				"pattern": pattern,
			}))
		}
		files = append(files, matches...)
	}
	return files, nil
}
