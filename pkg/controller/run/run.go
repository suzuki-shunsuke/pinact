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
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
)

type ParamRun struct {
	WorkflowFilePaths []string
	ConfigFilePath    string
	PWD               string
	IsVerify          bool
	Update            bool
	Check             bool
}

func (c *Controller) Run(ctx context.Context, logE *logrus.Entry) error {
	if err := c.readConfig(); err != nil {
		return err
	}
	workflowFilePaths, err := c.searchFiles()
	if err != nil {
		return fmt.Errorf("search target files: %w", err)
	}

	failed := false
	for _, workflowFilePath := range workflowFilePaths {
		logE := logE.WithField("workflow_file", workflowFilePath)
		if err := c.runWorkflow(ctx, logE, workflowFilePath); err != nil {
			if c.param.Check {
				failed = true
				if !errors.Is(err, ErrNotPinned) {
					logerr.WithError(logE, err).Error("check a workflow")
				}
				continue
			}
			failed = true
			if errors.Is(err, ErrNotPinned) {
				continue
			}
			logerr.WithError(logE, err).Error("update a workflow")
		}
	}
	if failed {
		return ErrNotPinned
	}
	return nil
}

func (c *Controller) readConfig() error {
	p, err := c.cfgFinder.Find(c.param.ConfigFilePath)
	if err != nil {
		return fmt.Errorf("find a configurationfile: %w", err)
	}
	c.param.ConfigFilePath = p
	cfg := &config.Config{}
	if err := c.cfgReader.Read(cfg, c.param.ConfigFilePath); err != nil {
		return fmt.Errorf("read a config file: %w", err)
	}
	c.cfg = cfg
	return nil
}

var ErrNotPinned = errors.New("actions aren't pinned")

func (c *Controller) runWorkflow(ctx context.Context, logE *logrus.Entry, workflowFilePath string) error { //nolint:cyclop
	lines, err := c.readWorkflow(workflowFilePath)
	if err != nil {
		return err
	}
	changed := false
	failed := false
	for i, line := range lines {
		l, err := c.parseLine(ctx, logE, line)
		if err != nil {
			logerr.WithError(logE, err).Error("parse a line")
			failed = true
			continue
		}
		if l == "" || line == l {
			continue
		}
		changed = true
		lines[i] = l
	}
	if c.param.Check && failed {
		return ErrNotPinned
	}
	if !changed {
		if failed {
			return ErrNotPinned
		}
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
	if failed {
		return ErrNotPinned
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
