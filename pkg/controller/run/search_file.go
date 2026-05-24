package run

import (
	"fmt"
	"path"
	"path/filepath"
)

// searchFiles determines which files to process based on configuration.
//
// When ParamRun.DiffFilter is nil (the normal case), it resolves the
// candidate set from explicit args, configured file patterns, or default
// discovery -- all via filepath.Glob against the local filesystem.
//
// When ParamRun.DiffFilter is set, the candidate set always starts from the
// files present in the diff (DiffFilter.Files()). It is then narrowed using
// path.Match -- never touching the disk -- so that `pinact run --diff-file`
// works in CI jobs that have not checked out the workflow files. The filter
// is the intersection of the diff with:
//   - the explicit args (WorkflowFilePaths), if any; otherwise
//   - the configured cfg.Files patterns (relative to the config dir), if any;
//     otherwise
//   - the package default patterns (defaultWorkflowPatterns).
//
// Returns a slice of file paths to process and any error encountered.
func (c *Controller) searchFiles() ([]string, error) {
	if c.param.DiffFilter == nil {
		return c.searchFilesFromDisk()
	}
	return c.searchFilesFromDiff()
}

// searchFilesFromDisk is the non-diff path: it resolves the candidate set by
// walking the local filesystem (explicit args > config patterns > defaults).
func (c *Controller) searchFilesFromDisk() ([]string, error) {
	if len(c.param.WorkflowFilePaths) != 0 {
		return c.param.WorkflowFilePaths, nil
	}
	if c.cfg != nil && len(c.cfg.Files) > 0 {
		return c.searchFilesByGlob()
	}
	return listWorkflows()
}

// searchFilesFromDiff intersects the diff's file set with the configured
// target (args / cfg.Files / defaultWorkflowPatterns) using path.Match.
// Diff paths are already slash-delimited (ParseDiff normalises them), which
// matches path.Match's separator convention.
func (c *Controller) searchFilesFromDiff() ([]string, error) {
	diffFiles := c.param.DiffFilter.Files()

	if len(c.param.WorkflowFilePaths) != 0 {
		allowed := make(map[string]struct{}, len(c.param.WorkflowFilePaths))
		for _, p := range c.param.WorkflowFilePaths {
			allowed[filepath.ToSlash(p)] = struct{}{}
		}
		filtered := make([]string, 0, len(diffFiles))
		for _, f := range diffFiles {
			if _, ok := allowed[f]; ok {
				filtered = append(filtered, f)
			}
		}
		return filtered, nil
	}

	if c.cfg != nil && len(c.cfg.Files) > 0 {
		configFileDir := filepath.ToSlash(filepath.Dir(c.param.ConfigFilePath))
		patterns := make([]string, 0, len(c.cfg.Files))
		for _, file := range c.cfg.Files {
			patterns = append(patterns, path.Join(configFileDir, file.Pattern))
		}
		return matchAny(patterns, diffFiles)
	}

	return matchAny(defaultWorkflowPatterns, diffFiles)
}

// matchAny returns the subset of files matching at least one of patterns,
// evaluated with path.Match. It preserves the input order of files and does
// not deduplicate (the input is expected to come from DiffFilter.Files(),
// which already has unique keys).
func matchAny(patterns, files []string) ([]string, error) {
	out := make([]string, 0, len(files))
	for _, f := range files {
		for _, p := range patterns {
			ok, err := path.Match(p, f)
			if err != nil {
				return nil, fmt.Errorf("match diff file against pattern %q: %w", p, err)
			}
			if ok {
				out = append(out, f)
				break
			}
		}
	}
	return out, nil
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
