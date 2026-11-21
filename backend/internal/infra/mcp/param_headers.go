package mcp

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const maxSafeJSONInteger = 1<<53 - 1

// headerParam 是 inputSchema 中以 x-mcp-header 标注、需要镜像为 Mcp-Param-{Name} 头的参数。
type headerParam struct {
	Name string
	Path []string
}

// parseHeaderParams 解析并校验 inputSchema 中的 x-mcp-header 标注。标注只能位于从根经 properties 链静态可达、
// 类型为 string / integer / boolean 的属性上，名称须是 HTTP token 且大小写不敏感唯一；任一违规都使整个工具定义无效。
func parseHeaderParams(schema json.RawMessage) ([]headerParam, error) {
	if len(strings.TrimSpace(string(schema))) == 0 {
		return nil, nil
	}
	var root any
	if err := json.Unmarshal(schema, &root); err != nil {
		return nil, nil
	}
	collector := headerParamCollector{seen: map[string]struct{}{}}
	collector.walk(root, nil, true)
	if collector.err != nil {
		return nil, collector.err
	}
	return collector.params, nil
}

type headerParamCollector struct {
	params []headerParam
	seen   map[string]struct{}
	err    error
}

func (c *headerParamCollector) walk(node any, path []string, reachable bool) {
	if c.err != nil {
		return
	}
	switch typed := node.(type) {
	case []any:
		for _, item := range typed {
			c.walk(item, path, false)
		}
	case map[string]any:
		if raw, ok := typed["x-mcp-header"]; ok {
			c.record(typed, raw, path, reachable)
		}
		for key, child := range typed {
			switch key {
			case "x-mcp-header":
			case "properties":
				properties, ok := child.(map[string]any)
				if !ok {
					c.walk(child, path, false)
					continue
				}
				for name, propertySchema := range properties {
					c.walk(propertySchema, appendPath(path, name), reachable)
				}
			default:
				c.walk(child, path, false)
			}
		}
	}
}

func (c *headerParamCollector) record(schema map[string]any, raw any, path []string, reachable bool) {
	name, _ := raw.(string)
	switch {
	case !reachable || len(path) == 0:
		c.err = fmt.Errorf("x-mcp-header %q is not on a statically reachable property", name)
	case !isHTTPToken(name):
		c.err = fmt.Errorf("x-mcp-header %q is not a valid header name", name)
	case !isHeaderParamType(schema["type"]):
		c.err = fmt.Errorf("x-mcp-header %q must annotate a string, integer or boolean property", name)
	default:
		key := strings.ToLower(name)
		if _, exists := c.seen[key]; exists {
			c.err = fmt.Errorf("x-mcp-header %q is declared more than once", name)
			return
		}
		c.seen[key] = struct{}{}
		c.params = append(c.params, headerParam{Name: name, Path: path})
	}
}

func appendPath(path []string, name string) []string {
	next := make([]string, len(path)+1)
	copy(next, path)
	next[len(path)] = name
	return next
}

func isHeaderParamType(value any) bool {
	switch value {
	case "string", "integer", "boolean":
		return true
	}
	return false
}

// isHTTPToken 校验 RFC 9110 field-name token 语法（1*tchar）。
func isHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
		case strings.ContainsRune("!#$%&'*+-.^_`|~", char):
		default:
			return false
		}
	}
	return true
}

// buildParamHeaders 从调用参数中取出 x-mcp-header 标注属性的值并编码；缺失或为 null 的值省略对应头。
func buildParamHeaders(params []headerParam, args map[string]any) map[string]string {
	if len(params) == 0 {
		return nil
	}
	headers := make(map[string]string, len(params))
	for _, param := range params {
		value, ok := valueAtPath(args, param.Path)
		if !ok {
			continue
		}
		if text, ok := headerParamText(value); ok {
			headers[param.Name] = encodeHeaderValue(text)
		}
	}
	return headers
}

func valueAtPath(args map[string]any, path []string) (any, bool) {
	var current any = args
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[key]
		if !ok {
			return nil, false
		}
	}
	return current, current != nil
}

func headerParamText(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case bool:
		return strconv.FormatBool(typed), true
	case int64:
		return strconv.FormatInt(typed, 10), typed >= -maxSafeJSONInteger && typed <= maxSafeJSONInteger
	case float64:
		if typed != math.Trunc(typed) || math.Abs(typed) > maxSafeJSONInteger {
			return "", false
		}
		return strconv.FormatInt(int64(typed), 10), true
	}
	return "", false
}
