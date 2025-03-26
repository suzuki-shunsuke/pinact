package migrate

import (
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/sirupsen/logrus"
)

func parseConfigAST(_ *logrus.Entry, content []byte) (string, error) {
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
