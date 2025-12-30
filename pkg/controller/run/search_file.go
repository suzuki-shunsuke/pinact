package run

import (
	"fmt"
	"path/filepath"
)

// searchFiles determines which files to process based on configuration.
// It returns workflow file paths from command line arguments if provided,
// otherwise uses configured file patterns, or falls back to default discovery.
//
// Returns a slice of file paths to process and any error encountered.
func (c *Controller) searchFiles() ([]string, error) {
	if len(c.param.WorkflowFilePaths) != 0 {
		return c.param.WorkflowFilePaths, nil
	}
	if c.cfg != nil && len(c.cfg.Files) > 0 {
		return c.searchFilesByGlob()
	}
	return ListWorkflows()
}

// searchFilesByGlob finds files using glob patterns from configuration.
// It applies each configured file pattern as a glob relative to the
// configuration file directory and collects all matching files.
//
// Returns a slice of matching file paths and any error encountered.
func (c *Controller) searchFilesByGlob() ([]string, error) {
	files := []string{}
	configFileDir := filepath.Dir(c.param.ConfigFilePath)
	for _, file := range c.cfg.Files {
		matches, err := filepath.Glob(filepath.Join(configFileDir, file.Pattern))
		if err != nil {
			return nil, fmt.Errorf("search target files: %w", err)
		}
		files = append(files, matches...)
	}
	return files, nil
}
