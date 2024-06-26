package run

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

func (c *Controller) searchFiles(logE *logrus.Entry, workflowFilePaths []string, cfg *Config, pwd string) ([]string, error) {
	if len(workflowFilePaths) != 0 {
		return workflowFilePaths, nil
	}
	if len(cfg.Files) > 0 {
		return c.searchFilesByConfig(logE, cfg, pwd)
	}
	return listWorkflows()
}

func (c *Controller) searchFilesByConfig(logE *logrus.Entry, cfg *Config, pwd string) ([]string, error) {
	patterns := make([]*regexp.Regexp, 0, len(cfg.Files))
	for _, file := range cfg.Files {
		if file.Pattern == "" {
			// ignore
			continue
		}
		p, err := regexp.Compile(file.Pattern)
		if err != nil {
			return nil, fmt.Errorf("parse files[].pattern as a regular expression: %w", err)
		}
		patterns = append(patterns, p)
	}

	files := []string{}
	if err := fs.WalkDir(afero.NewIOFS(c.fs), pwd, func(p string, dirEntry fs.DirEntry, e error) error {
		if e != nil {
			return nil //nolint:nilerr
		}
		if dirEntry.IsDir() {
			// ignore directory
			return nil
		}
		filePath, err := filepath.Rel(pwd, p)
		if err != nil {
			logE.WithFields(logrus.Fields{
				"pwd":  pwd,
				"path": p,
			}).WithError(err).Debug("get a relative path")
			return nil
		}
		for _, pattern := range patterns {
			if pattern.MatchString(filePath) {
				files = append(files, filePath)
				break
			}
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("search target files: %w", err)
	}

	return files, nil
}
