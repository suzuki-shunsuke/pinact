package run

import (
	"fmt"
	"path/filepath"
)

func listWorkflows() ([]string, error) {
	files, err := filepath.Glob(".github/workflows/*.yml")
	if err != nil {
		return nil, fmt.Errorf("find .github/workflows/*.yml: %w", err)
	}
	files2, err := filepath.Glob(".github/workflows/*.yaml")
	if err != nil {
		return nil, fmt.Errorf("find .github/workflows/*.yaml: %w", err)
	}
	return append(files, files2...), nil
}
