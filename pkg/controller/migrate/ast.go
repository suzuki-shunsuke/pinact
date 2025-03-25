package migrate

import (
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/sirupsen/logrus"
)

const falseStr = "false"

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
	//     name_format:
	//     ref:
	//     ref_format:
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
		return errors.New("version must be an array")
	}
}

func migrateIgnoreAction(body ast.Node) error {
	// name:
	// name_format:
	// ref:
	// ref_format:
	m, ok := body.(*ast.MappingNode)
	if !ok {
		return errors.New("ignore_action must be a mapping node")
	}

	value := map[string]any{}

	nameFormatNode := findNodeByKey(m.Values, "name_format")
	if nameFormatNode == nil {
		value["name_format"] = "regexp"
	}

	if refNode := findNodeByKey(m.Values, "ref"); refNode != nil {
		refFormatNode := findNodeByKey(m.Values, "ref_format")
		if refFormatNode == nil {
			value["ref_format"] = "regexp"
		}
	}

	if len(value) == 0 {
		return nil
	}
	node, err := yaml.ValueToNode(value)
	if err != nil {
		return fmt.Errorf("convert name_format to node: %w", err)
	}
	m.Merge(node.(*ast.MappingNode)) //nolint:forcetypeassert
	return nil
}

func migrateVersion(body *ast.MappingNode) error {
	// version:
	versionNode := findNodeByKey(body.Values, "version")
	if versionNode == nil {
		node, err := yaml.ValueToNode(map[string]any{
			"version": 3,
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
