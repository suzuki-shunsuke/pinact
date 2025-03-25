package run

import (
	"fmt"
	"path/filepath"
)

func (c *Controller) searchFiles() ([]string, error) {
	if len(c.param.WorkflowFilePaths) != 0 {
		return c.param.WorkflowFilePaths, nil
	}
	if len(c.cfg.Files) > 0 {
		return c.searchFilesByConfig()
	}
	return listWorkflows()
}

func (c *Controller) searchFilesByConfig() ([]string, error) {
	files := []string{}
	for _, file := range c.cfg.Files {
		matches, err := filepath.Glob(file.Pattern)
		if err != nil {
			return nil, fmt.Errorf("search target files: %w", err)
		}
		files = append(files, matches...)
	}
	return files, nil
}
