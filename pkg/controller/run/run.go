package run

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/logrus-error/logerr"
)

type ParamRun struct {
	WorkflowFilePaths []string
	ConfigFilePath    string
	PWD               string
	IsVerify          bool
	Update            bool
	Check             bool
}

func (c *Controller) Run(ctx context.Context, logE *logrus.Entry, param *ParamRun) error {
	cfg := &Config{}
	if err := c.readConfig(param.ConfigFilePath, cfg); err != nil {
		return err
	}
	cfg.IsVerify = param.IsVerify
	cfg.Check = param.Check
	workflowFilePaths, err := c.searchFiles(logE, param.WorkflowFilePaths, cfg, param.PWD)
	if err != nil {
		return fmt.Errorf("search target files: %w", err)
	}

	failed := false
	for _, workflowFilePath := range workflowFilePaths {
		logE := logE.WithField("workflow_file", workflowFilePath)
		if err := c.runWorkflow(ctx, logE, workflowFilePath, cfg); err != nil {
			if param.Check {
				failed = true
				if !errors.Is(err, ErrNotPinned) {
					logerr.WithError(logE, err).Error("check a workflow")
				}
				continue
			}
			logerr.WithError(logE, err).Warn("update a workflow")
		}
	}
	if failed {
		return ErrNotPinned
	}
	return nil
}

var ErrNotPinned = errors.New("actions aren't pinned")

func (c *Controller) runWorkflow(ctx context.Context, logE *logrus.Entry, workflowFilePath string, cfg *Config) error {
	lines, err := c.readWorkflow(workflowFilePath)
	if err != nil {
		return err
	}
	changed := false
	failed := false
	for i, line := range lines {
		l, err := c.parseLine(ctx, logE, line, cfg)
		if err != nil {
			logerr.WithError(logE, err).Error("parse a line")
			if cfg.Check {
				failed = true
			}
			continue
		}
		if line == l {
			continue
		}
		changed = true
		lines[i] = l
	}
	if failed {
		return ErrNotPinned
	}
	if !changed {
		return nil
	}
	f, err := os.Create(workflowFilePath)
	if err != nil {
		return fmt.Errorf("create a workflow file: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(strings.Join(lines, "\n") + "\n"); err != nil {
		return fmt.Errorf("write a workflow file: %w", err)
	}
	return nil
}

func (c *Controller) readWorkflow(workflowFilePath string) ([]string, error) {
	workflowReadFile, err := os.Open(workflowFilePath)
	if err != nil {
		return nil, fmt.Errorf("open a workflow file: %w", err)
	}
	defer workflowReadFile.Close()
	scanner := bufio.NewScanner(workflowReadFile)
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan a workflow file: %w", err)
	}
	return lines, nil
}
