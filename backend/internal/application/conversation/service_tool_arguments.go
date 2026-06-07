package conversation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

type toolArgumentIssue struct {
	path     string
	message  string
	expected string
	received string
}

type toolArgumentValidationError struct {
	issues []toolArgumentIssue
}

type toolArgumentEnumNumber struct {
	value string
}

func (e toolArgumentValidationError) Error() string {
	if len(e.issues) == 0 {
		return "tool argument validation failed"
	}
	parts := make([]string, 0, len(e.issues))
	for _, issue := range e.issues {
		if issue.message != "" {
			parts = append(parts, issue.message)
			continue
		}
		if issue.path == "" {
			parts = append(parts, fmt.Sprintf("tool arguments must be %s, received %s", issue.expected, issue.received))
			continue
		}
		parts = append(parts, fmt.Sprintf("parameter `%s` must be %s, received %s", issue.path, issue.expected, issue.received))
	}
	return "tool argument validation failed: " + strings.Join(parts, "; ")
}

func normalizeToolArguments(raw string, schema json.RawMessage) (string, error) {
	args, err := decodeToolArgumentObject(raw)
	if err != nil {
		return "", err
	}
	normalizedSchema, schemaOK := decodeToolArgumentSchema(schema)
	if schemaOK {
		if issues := validateToolArgumentValue(args, normalizedSchema, ""); len(issues) > 0 {
			return "", toolArgumentValidationError{issues: issues}
		}
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func decodeToolArgumentObject(raw string) (map[string]interface{}, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return map[string]interface{}{}, nil
	}
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	var parsed interface{}
	if err := decoder.Decode(&parsed); err != nil {
		return nil, toolArgumentValidationError{issues: []toolArgumentIssue{{
			message: "tool arguments must be a valid JSON object",
		}}}
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, toolArgumentValidationError{issues: []toolArgumentIssue{{
			message: "tool arguments must contain one JSON object",
		}}}
	}
	object, ok := parsed.(map[string]interface{})
	if !ok {
		return nil, toolArgumentValidationError{issues: []toolArgumentIssue{{
			expected: "object",
			received: toolArgumentTypeName(parsed),
		}}}
	}
	return object, nil
}

func decodeToolArgumentSchema(raw json.RawMessage) (map[string]interface{}, bool) {
	value := bytes.TrimSpace(raw)
	if len(value) == 0 {
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.UseNumber()
	var schema map[string]interface{}
	if err := decoder.Decode(&schema); err != nil || len(schema) == 0 {
		return nil, false
	}
	if _, ok := schema["type"]; !ok {
		if _, hasProperties := schema["properties"]; hasProperties {
			schema["type"] = "object"
		}
	}
	return schema, true
}

func validateToolArgumentValue(value interface{}, schema map[string]interface{}, path string) []toolArgumentIssue {
	if len(schema) == 0 {
		return nil
	}
	expectedTypes := toolArgumentSchemaTypes(schema)
	if len(expectedTypes) == 0 {
		if _, hasProperties := schema["properties"]; hasProperties {
			expectedTypes = []string{"object"}
		}
	}
	if len(expectedTypes) == 0 {
		return nil
	}

	var lastIssues []toolArgumentIssue
	for _, expectedType := range expectedTypes {
		if expectedType == "null" && value == nil {
			return nil
		}
		trialValue, issues := validateToolArgumentTypedValue(value, schema, expectedType, path)
		if len(issues) == 0 {
			_ = trialValue
			return nil
		}
		lastIssues = issues
	}
	return lastIssues
}

func validateToolArgumentTypedValue(value interface{}, schema map[string]interface{}, expectedType string, path string) (interface{}, []toolArgumentIssue) {
	coerced, ok := coerceToolArgumentValue(value, expectedType)
	if !ok {
		return value, []toolArgumentIssue{{
			path:     path,
			expected: expectedType,
			received: toolArgumentTypeName(value),
		}}
	}

	switch expectedType {
	case "object":
		object, ok := coerced.(map[string]interface{})
		if !ok {
			return coerced, []toolArgumentIssue{{path: path, expected: "object", received: toolArgumentTypeName(value)}}
		}
		if issues := validateToolArgumentObject(object, schema, path); len(issues) > 0 {
			return object, issues
		}
		if issue, ok := validateToolArgumentEnum(object, schema, path); !ok {
			return object, []toolArgumentIssue{issue}
		}
		return object, nil
	case "array":
		items, ok := coerced.([]interface{})
		if !ok {
			return coerced, []toolArgumentIssue{{path: path, expected: "array", received: toolArgumentTypeName(value)}}
		}
		itemSchema, _ := schema["items"].(map[string]interface{})
		for index, item := range items {
			childPath := joinToolArgumentPath(path, strconv.Itoa(index), true)
			if childIssues := validateToolArgumentValue(item, itemSchema, childPath); len(childIssues) > 0 {
				return items, childIssues
			}
			if normalized, ok := normalizeToolArgumentProperty(item, itemSchema); ok {
				items[index] = normalized
			}
		}
		if issue, ok := validateToolArgumentEnum(items, schema, path); !ok {
			return items, []toolArgumentIssue{issue}
		}
		return items, nil
	default:
		if issue, ok := validateToolArgumentEnum(coerced, schema, path); !ok {
			return coerced, []toolArgumentIssue{issue}
		}
		return coerced, nil
	}
}

func validateToolArgumentObject(object map[string]interface{}, schema map[string]interface{}, path string) []toolArgumentIssue {
	required := toolArgumentRequiredFields(schema)
	for _, name := range required {
		if _, ok := object[name]; !ok {
			return []toolArgumentIssue{{
				path:    joinToolArgumentPath(path, name, false),
				message: fmt.Sprintf("required parameter `%s` is missing", joinToolArgumentPath(path, name, false)),
			}}
		}
	}
	properties, _ := schema["properties"].(map[string]interface{})
	for name, rawPropertySchema := range properties {
		propertySchema, ok := rawPropertySchema.(map[string]interface{})
		if !ok {
			continue
		}
		item, ok := object[name]
		if !ok {
			continue
		}
		childPath := joinToolArgumentPath(path, name, false)
		if issues := validateToolArgumentValue(item, propertySchema, childPath); len(issues) > 0 {
			return issues
		}
		if normalized, ok := normalizeToolArgumentProperty(item, propertySchema); ok {
			object[name] = normalized
		}
	}
	return nil
}

func normalizeToolArgumentProperty(value interface{}, schema map[string]interface{}) (interface{}, bool) {
	for _, expectedType := range toolArgumentSchemaTypes(schema) {
		if normalized, ok := coerceToolArgumentValue(value, expectedType); ok {
			return normalized, true
		}
	}
	return value, false
}

func coerceToolArgumentValue(value interface{}, expectedType string) (interface{}, bool) {
	switch expectedType {
	case "object":
		_, ok := value.(map[string]interface{})
		return value, ok
	case "array":
		_, ok := value.([]interface{})
		return value, ok
	case "string":
		_, ok := value.(string)
		return value, ok
	case "boolean":
		switch typed := value.(type) {
		case bool:
			return typed, true
		case string:
			switch strings.ToLower(strings.TrimSpace(typed)) {
			case "true":
				return true, true
			case "false":
				return false, true
			}
		}
		return value, false
	case "integer":
		switch typed := value.(type) {
		case json.Number:
			parsed, err := strconv.ParseInt(typed.String(), 10, 64)
			return parsed, err == nil
		case float64:
			if math.Trunc(typed) == typed {
				return int64(typed), true
			}
		case string:
			parsed, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
			return parsed, err == nil
		}
		return value, false
	case "number":
		switch typed := value.(type) {
		case json.Number:
			parsed, err := strconv.ParseFloat(typed.String(), 64)
			return parsed, err == nil
		case float64:
			return typed, true
		case string:
			parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
			return parsed, err == nil
		}
		return value, false
	default:
		return value, true
	}
}

func toolArgumentSchemaTypes(schema map[string]interface{}) []string {
	switch raw := schema["type"].(type) {
	case string:
		if value := strings.TrimSpace(raw); value != "" {
			return []string{value}
		}
	case []interface{}:
		types := make([]string, 0, len(raw))
		for _, item := range raw {
			if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
				types = append(types, strings.TrimSpace(value))
			}
		}
		return types
	}
	return nil
}

func toolArgumentRequiredFields(schema map[string]interface{}) []string {
	items, _ := schema["required"].([]interface{})
	result := make([]string, 0, len(items))
	for _, item := range items {
		name, ok := item.(string)
		if !ok || strings.TrimSpace(name) == "" {
			continue
		}
		result = append(result, strings.TrimSpace(name))
	}
	return result
}

func toolArgumentMatchesEnum(value interface{}, enumValues []interface{}) bool {
	normalizedValue := normalizeEnumComparableValue(value)
	for _, item := range enumValues {
		if reflect.DeepEqual(normalizedValue, normalizeEnumComparableValue(item)) {
			return true
		}
	}
	return false
}

func validateToolArgumentEnum(value interface{}, schema map[string]interface{}, path string) (toolArgumentIssue, bool) {
	enumValues, ok := schema["enum"].([]interface{})
	if !ok || len(enumValues) == 0 {
		return toolArgumentIssue{}, true
	}
	if toolArgumentMatchesEnum(value, enumValues) {
		return toolArgumentIssue{}, true
	}
	return toolArgumentIssue{
		path:     path,
		expected: "one of the declared enum values",
		received: toolArgumentTypeName(value),
	}, false
}

func normalizeEnumComparableValue(value interface{}) interface{} {
	if number, ok := normalizeEnumComparableNumber(value); ok {
		return number
	}
	switch typed := value.(type) {
	case map[string]interface{}:
		next := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			next[key] = normalizeEnumComparableValue(item)
		}
		return next
	case []interface{}:
		next := make([]interface{}, len(typed))
		for index, item := range typed {
			next[index] = normalizeEnumComparableValue(item)
		}
		return next
	default:
		return value
	}
}

func normalizeEnumComparableNumber(value interface{}) (toolArgumentEnumNumber, bool) {
	switch typed := value.(type) {
	case json.Number:
		return normalizeEnumComparableNumberString(typed.String())
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return toolArgumentEnumNumber{}, false
		}
		return normalizeEnumComparableNumberString(strconv.FormatFloat(typed, 'g', -1, 64))
	case int:
		return normalizeEnumComparableNumberString(strconv.FormatInt(int64(typed), 10))
	case int64:
		return normalizeEnumComparableNumberString(strconv.FormatInt(typed, 10))
	default:
		return toolArgumentEnumNumber{}, false
	}
}

func normalizeEnumComparableNumberString(raw string) (toolArgumentEnumNumber, bool) {
	rational, ok := new(big.Rat).SetString(strings.TrimSpace(raw))
	if !ok {
		return toolArgumentEnumNumber{}, false
	}
	return toolArgumentEnumNumber{value: rational.RatString()}, true
}

func toolArgumentTypeName(value interface{}) string {
	switch value.(type) {
	case nil:
		return "null"
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case json.Number, float64, int, int64:
		return "number"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func joinToolArgumentPath(parent string, child string, index bool) string {
	if parent == "" {
		if index {
			return "[" + child + "]"
		}
		return child
	}
	if index {
		return parent + "[" + child + "]"
	}
	return parent + "." + child
}
