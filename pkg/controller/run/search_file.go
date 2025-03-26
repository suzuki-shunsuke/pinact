package run

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

func (c *Controller) searchFiles(logE *logrus.Entry) ([]string, error) {
	if len(c.param.WorkflowFilePaths) != 0 {
		return c.param.WorkflowFilePaths, nil
	}
	if c.cfg != nil && len(c.cfg.Files) > 0 {
		return c.searchFilesByConfig(logE)
	}
	return listWorkflows()
}

func (c *Controller) searchFilesByConfig(logE *logrus.Entry) ([]string, error) {
	switch c.cfg.Version {
	case 0, 2: //nolint:mnd
		return c.searchFilesByRegexp(logE)
	case 3: //nolint:mnd
		return c.searchFilesByGlob()
	default:
		return nil, fmt.Errorf("unsupported version %d", c.cfg.Version)
	}
}

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

func (c *Controller) searchFilesByRegexp(logE *logrus.Entry) ([]string, error) {
	patterns := make([]*regexp.Regexp, 0, len(c.cfg.Files))
	configFileDir := filepath.Dir(c.param.ConfigFilePath)
	for _, file := range c.cfg.Files {
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
	if err := fs.WalkDir(afero.NewIOFS(c.fs), configFileDir, func(p string, dirEntry fs.DirEntry, e error) error {
		if e != nil {
			return nil //nolint:nilerr
		}
		if dirEntry.IsDir() {
			// ignore directory
			return nil
		}
		filePath, err := filepath.Rel(configFileDir, p)
		if err != nil {
			logE.WithFields(logrus.Fields{
				"config_file_dir": configFileDir,
				"path":            p,
			}).WithError(err).Debug("get a relative path")
			return nil
		}
		sp := filepath.ToSlash(filePath)
		for _, pattern := range patterns {
			if pattern.MatchString(sp) {
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
