package list

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/run"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
)

// List executes the main list operation.
// It searches for workflow files and outputs action information.
func (c *Controller) List(ctx context.Context, logger *slog.Logger) error {
	workflowFilePaths, err := c.searchFiles()
	if err != nil {
		return fmt.Errorf("search target files: %w", err)
	}

	tmpl, err := c.parseTemplate()
	if err != nil {
		return err
	}

	for _, workflowFilePath := range workflowFilePaths {
		logger := logger.With("workflow_file", workflowFilePath)
		if err := c.listWorkflow(ctx, logger, workflowFilePath, tmpl); err != nil {
			slogerr.WithError(logger, err).Error("list actions in workflow")
		}
	}
	return nil
}

func (c *Controller) parseTemplate() (*template.Template, error) {
	if c.param.LineTemplate == "" {
		return nil, nil //nolint:nilnil
	}
	tmpl, err := template.New("line").Parse(c.param.LineTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse line template: %w", err)
	}
	return tmpl, nil
}

func (c *Controller) searchFiles() ([]string, error) {
	if len(c.param.WorkflowFilePaths) != 0 {
		return c.param.WorkflowFilePaths, nil
	}
	if c.cfg != nil && len(c.cfg.Files) > 0 {
		return c.searchFilesByGlob()
	}
	files, err := run.ListWorkflows()
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}
	return files, nil
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

func (c *Controller) listWorkflow(_ context.Context, logger *slog.Logger, workflowFilePath string, tmpl *template.Template) error {
	lines, err := c.readWorkflow(workflowFilePath)
	if err != nil {
		return err
	}

	for i, line := range lines {
		lineNumber := i + 1
		action := run.ParseAction(line)
		if action == nil {
			continue
		}

		// Parse action name to get owner and repo
		if !c.parseActionName(action) {
			continue
		}

		// Apply include/exclude filters
		if c.excludeAction(action.Name) {
			logger.Debug("exclude the action", "action", action.Name)
			continue
		}
		if c.excludeByIncludes(action.Name) {
			logger.Debug("exclude the action by includes", "action", action.Name)
			continue
		}

		// Apply owner filter
		if c.param.Owner != "" && action.RepoOwner != c.param.Owner {
			continue
		}

		info := &ActionInfo{
			ActionName: action.Name,
			RepoOwner:  action.RepoOwner,
			RepoName:   action.RepoName,
			Version:    action.Version,
			Comment:    action.VersionComment,
			FilePath:   workflowFilePath,
			FileName:   filepath.Base(workflowFilePath),
			LineNumber: lineNumber,
		}

		if err := c.output(info, tmpl); err != nil {
			return err
		}
	}
	return nil
}

func (c *Controller) output(info *ActionInfo, tmpl *template.Template) error {
	if tmpl != nil {
		if err := tmpl.Execute(c.stdout, info); err != nil {
			return fmt.Errorf("execute template: %w", err)
		}
		fmt.Fprintln(c.stdout)
		return nil
	}
	// Default CSV format: <FilePath>,<LineNumber>,<ActionName>,<Version>,<Comment>
	fmt.Fprintf(c.stdout, "%s,%d,%s,%s,%s\n", info.FilePath, info.LineNumber, info.ActionName, info.Version, info.Comment)
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

func (c *Controller) parseActionName(action *run.Action) bool {
	a := strings.Split(action.Name, "/")
	if len(a) == 1 {
		return false
	}
	action.RepoOwner = a[0]
	action.RepoName = a[1]
	return true
}

func (c *Controller) excludeAction(actionName string) bool {
	for _, exclude := range c.param.Excludes {
		if exclude.MatchString(actionName) {
			return true
		}
	}
	return false
}

func (c *Controller) excludeByIncludes(actionName string) bool {
	if len(c.param.Includes) == 0 {
		return false
	}
	for _, include := range c.param.Includes {
		if include.MatchString(actionName) {
			return false
		}
	}
	return true
}
