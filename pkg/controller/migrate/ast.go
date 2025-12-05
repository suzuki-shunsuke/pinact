package migrate

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// parseConfigAST parses and migrates a YAML configuration file using AST.
// It parses the YAML content, applies migrations to each document,
// and returns the updated YAML content as a string.
//
// Parameters:
//   - _: slog logger (unused in current implementation)
//   - content: YAML configuration file content as bytes
//
// Returns the migrated YAML content as string and any error encountered.
func parseConfigAST(_ *slog.Logger, content []byte) (string, error) {
	file, err := parser.ParseBytes(content, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse a workflow file as YAML: %w", err)
	}
	for _, doc := range file.Docs {
		if err := parseDocAST(doc); err != nil {
			return "", err
		}
	}
	return file.String(), nil
}

// parseDocAST migrates a single YAML document node.
// It processes the document body to migrate ignore_actions and version fields
// to the latest configuration schema format.
//
// Parameters:
//   - doc: YAML document node to migrate
//
// Returns an error if migration fails.
func parseDocAST(doc *ast.DocumentNode) error {
	body, ok := doc.Body.(*ast.MappingNode)
	if !ok {
		return errors.New("document body must be *ast.MappingNode")
	}
	if err := migrateIgnoreActions(body); err != nil {
		return fmt.Errorf("migrate ignore_actions: %w", err)
	}
	if err := migrateVersion(body); err != nil {
		return fmt.Errorf("migrate version: %w", err)
	}
	return nil
}

// migrateIgnoreActions migrates the ignore_actions section of the configuration.
// It processes each ignore action to ensure they have required ref fields
// with default values if missing.
//
// Parameters:
//   - body: YAML mapping node containing the configuration
//
// Returns an error if migration fails.
func migrateIgnoreActions(body *ast.MappingNode) error {
	// ignore_actions:
	//   - name:
	//     ref:
	ignoreActionsNode := findNodeByKey(body.Values, "ignore_actions")
	if ignoreActionsNode == nil {
		return nil
	}
	switch seq := ignoreActionsNode.Value.(type) {
	case *ast.SequenceNode:
		for _, value := range seq.Values {
			if err := migrateIgnoreAction(value); err != nil {
				return fmt.Errorf("migrate ignore_actions: %w", err)
			}
		}
		return nil
	default:
		return errors.New("ignore_actions must be an array")
	}
}

// migrateIgnoreAction migrates a single ignore action configuration.
// It ensures the ignore action has a ref field, adding a default ".*"
// pattern if the ref field is missing.
//
// Parameters:
//   - body: YAML node representing an ignore action
//
// Returns an error if migration fails.
func migrateIgnoreAction(body ast.Node) error {
	// name:
	// ref:
	m, ok := body.(*ast.MappingNode)
	if !ok {
		return errors.New("ignore_action must be a mapping node")
	}

	if refNode := findNodeByKey(m.Values, "ref"); refNode != nil {
		return nil
	}

	node, err := yaml.ValueToNode(map[string]any{
		"ref": ".*",
	})
	if err != nil {
		return fmt.Errorf("convert ref to node: %w", err)
	}
	m.Merge(node.(*ast.MappingNode)) //nolint:forcetypeassert
	return nil
}

// migrateVersion migrates the version field of the configuration.
// It adds a version field if missing or updates an existing version
// to the current schema version (3).
//
// Parameters:
//   - body: YAML mapping node containing the configuration
//
// Returns an error if migration fails.
func migrateVersion(body *ast.MappingNode) error {
	// version:
	versionNode := findNodeByKey(body.Values, "version")
	if versionNode == nil {
		node, err := yaml.ValueToNode(map[string]any{
			"version": 3, //nolint:mnd
		})
		if err != nil {
			return fmt.Errorf("convert version to node: %w", err)
		}
		body.Merge(node.(*ast.MappingNode)) //nolint:forcetypeassert
		return nil
	}

	switch v := versionNode.Value.(type) {
	case *ast.IntegerNode:
		v.Token.Value = "3"
		v.Value = 3
		return nil
	default:
		return errors.New("version must be a number")
	}
}

func findNodeByKey(values []*ast.MappingValueNode, key string) *ast.MappingValueNode {
	for _, value := range values {
		k, ok := value.Key.(*ast.StringNode)
		if !ok {
			continue
		}
		if k.Value == key {
			return value
		}
	}
	return nil
}
