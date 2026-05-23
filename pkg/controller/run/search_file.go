package run

import (
	"fmt"
	"path/filepath"
)

// searchFiles determines which files to process based on configuration.
// It returns workflow file paths from command line arguments if provided,
// otherwise uses configured file patterns, or falls back to default discovery.
//
// When ParamRun.DiffFilter is set, the result is further intersected with the
// set of files appearing in the unified diff. DiffFilter normalizes lookup
// paths via filepath.ToSlash internally, so OS-native paths (e.g. Windows
// backslashes from filepath.Glob) match the slash-delimited diff keys. The
// comparison still assumes the diff's paths and the discovery results share
// the same root (typically: pinact is invoked from the repository root).
//
// Returns a slice of file paths to process and any error encountered.
func (c *Controller) searchFiles() ([]string, error) {
	files, err := c.searchFilesRaw()
	if err != nil {
		return nil, err
	}
	if c.param.DiffFilter == nil {
		return files, nil
	}
	filtered := files[:0]
	for _, f := range files {
		if c.param.DiffFilter.Has(f) {
			filtered = append(filtered, f)
		}
	}
	return filtered, nil
}

// searchFilesRaw performs the unfiltered discovery step shared by all modes.
func (c *Controller) searchFilesRaw() ([]string, error) {
	if len(c.param.WorkflowFilePaths) != 0 {
		return c.param.WorkflowFilePaths, nil
	}
	if c.cfg != nil && len(c.cfg.Files) > 0 {
		return c.searchFilesByGlob()
	}
	return listWorkflows()
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
