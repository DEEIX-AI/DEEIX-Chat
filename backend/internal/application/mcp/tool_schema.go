package mcp

import (
	"encoding/json"
	"strings"
)

const maxToolSchemaReferenceDepth = 32

func toolSchemaPropertyAcceptsString(schemaJSON string, propertyName string) bool {
	var root map[string]any
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(schemaJSON)))
	decoder.UseNumber()
	if err := decoder.Decode(&root); err != nil || len(root) == 0 {
		return false
	}
	properties, ok := root["properties"].(map[string]any)
	if !ok {
		return false
	}
	property, ok := properties[propertyName]
	if !ok {
		return false
	}
	return jsonSchemaNodeAcceptsString(root, property, map[string]struct{}{}, 0)
}

func jsonSchemaNodeAcceptsString(
	root map[string]any,
	node any,
	resolving map[string]struct{},
	depth int,
) bool {
	if depth > maxToolSchemaReferenceDepth {
		return false
	}
	schema, ok := node.(map[string]any)
	if !ok {
		return false
	}

	if reference, ok := schema["$ref"].(string); ok {
		reference = strings.TrimSpace(reference)
		if reference == "" {
			return false
		}
		if _, cyclic := resolving[reference]; cyclic {
			return false
		}
		resolved, found := resolveLocalJSONSchemaReference(root, reference)
		if !found {
			return false
		}
		resolving[reference] = struct{}{}
		acceptsReference := jsonSchemaNodeAcceptsString(root, resolved, resolving, depth+1)
		delete(resolving, reference)
		if !acceptsReference {
			return false
		}
		if len(schema) == 1 {
			return true
		}
		siblings := make(map[string]any, len(schema)-1)
		for key, value := range schema {
			if key != "$ref" {
				siblings[key] = value
			}
		}
		return jsonSchemaNodeAcceptsString(root, siblings, resolving, depth+1)
	}

	if rawType, exists := schema["type"]; exists && !jsonSchemaTypeIncludesString(rawType) {
		return false
	}
	if constant, exists := schema["const"]; exists {
		_, stringConstant := constant.(string)
		if !stringConstant {
			return false
		}
	}
	if enumValues, exists := schema["enum"]; exists {
		items, valid := enumValues.([]any)
		if !valid {
			return false
		}
		stringOption := false
		for _, item := range items {
			if _, ok := item.(string); ok {
				stringOption = true
				break
			}
		}
		if !stringOption {
			return false
		}
	}
	for _, keyword := range []string{"anyOf", "oneOf"} {
		if branches, exists := schema[keyword]; exists {
			items, valid := branches.([]any)
			if !valid || len(items) == 0 {
				return false
			}
			accepted := false
			for _, branch := range items {
				if jsonSchemaNodeAcceptsString(root, branch, resolving, depth+1) {
					accepted = true
					break
				}
			}
			if !accepted {
				return false
			}
		}
	}
	if branches, exists := schema["allOf"]; exists {
		items, valid := branches.([]any)
		if !valid || len(items) == 0 {
			return false
		}
		for _, branch := range items {
			if !jsonSchemaNodeAcceptsString(root, branch, resolving, depth+1) {
				return false
			}
		}
	}
	return true
}

func jsonSchemaTypeIncludesString(value any) bool {
	switch item := value.(type) {
	case string:
		return item == "string"
	case []any:
		for _, candidate := range item {
			if candidate == "string" {
				return true
			}
		}
	}
	return false
}

func resolveLocalJSONSchemaReference(root map[string]any, reference string) (any, bool) {
	if reference == "#" {
		return root, true
	}
	if !strings.HasPrefix(reference, "#/") {
		return nil, false
	}
	var current any = root
	for _, rawSegment := range strings.Split(strings.TrimPrefix(reference, "#/"), "/") {
		segment := strings.ReplaceAll(strings.ReplaceAll(rawSegment, "~1", "/"), "~0", "~")
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
