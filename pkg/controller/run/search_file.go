package run

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
)

func (c *Controller) searchFiles(logE *logrus.Entry) ([]string, error) {
	if len(c.param.WorkflowFilePaths) != 0 {
		return c.param.WorkflowFilePaths, nil
	}
	if len(c.cfg.Files) > 0 {
		return c.searchFilesByConfig(logE)
	}
	return listWorkflows()
}

func (c *Controller) searchFilesByConfig(logE *logrus.Entry) ([]string, error) {
	files := []string{}
	for _, file := range c.cfg.Files {
		switch file.PatternFormat {
		case "fixed_string":
			files = append(files, file.Pattern)
			continue
		case "glob":
			matches, err := filepath.Glob(file.Pattern)
			if err != nil {
				return nil, fmt.Errorf("search target files: %w", err)
			}
			files = append(files, matches...)
		case "regexp":
			if err := fs.WalkDir(afero.NewIOFS(c.fs), c.param.PWD, func(p string, dirEntry fs.DirEntry, e error) error {
				if e != nil {
					return nil //nolint:nilerr
				}
				if dirEntry.IsDir() {
					// ignore directory
					return nil
				}
				filePath, err := filepath.Rel(c.param.PWD, p)
				if err != nil {
					logE.WithFields(logrus.Fields{
						"pwd":  c.param.PWD,
						"path": p,
					}).WithError(err).Debug("get a relative path")
					return nil
				}
				sp := filepath.ToSlash(filePath)
				for _, file := range c.cfg.Files {
					if file.patternRegexp.MatchString(sp) {
						files = append(files, filePath)
						break
					}
				}
				return nil
			}); err != nil {
				return nil, fmt.Errorf("search target files: %w", err)
			}
		default:
			return nil, fmt.Errorf("unexpected pattern format: %s", file.PatternFormat)
		}
	}

	return files, nil
}
