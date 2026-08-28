package conversation

import (
	"strconv"
	"strings"
)

// MCPToolUsageItem 聚合一次运行内某个 MCP 工具的成功调用计量。
// 只统计真正到达上游的成功调用：run 内去重复用（reused）、参数校验失败
// 和未启用错误都不产生上游费用，不进入计量。价格取调用时的工具配置快照。
type MCPToolUsageItem struct {
	ServerID     uint
	ServerName   string
	ToolName     string
	CallCount    int64
	PriceNanousd int64
}

// mcpToolUsageKey 以服务器与工具唯一标识一条聚合记录；
// 价格参与键值，保证同一 run 内价格变更前后的调用不会被错误合并。
func mcpToolUsageKey(item MCPToolUsageItem) string {
	return strconv.FormatUint(uint64(item.ServerID), 10) + "\x00" +
		strings.ToLower(strings.TrimSpace(item.ServerName)) + "\x00" +
		strings.ToLower(strings.TrimSpace(item.ToolName)) + "\x00" +
		strconv.FormatInt(item.PriceNanousd, 10)
}

// mergeMCPToolUsage 合并两批聚合记录，保持首次出现顺序稳定。
func mergeMCPToolUsage(left []MCPToolUsageItem, right []MCPToolUsageItem) []MCPToolUsageItem {
	if len(left) == 0 && len(right) == 0 {
		return nil
	}
	result := make([]MCPToolUsageItem, 0, len(left)+len(right))
	index := make(map[string]int, len(left)+len(right))
	appendItems := func(items []MCPToolUsageItem) {
		for _, item := range items {
			if item.CallCount <= 0 {
				continue
			}
			key := mcpToolUsageKey(item)
			if position, ok := index[key]; ok {
				result[position].CallCount += item.CallCount
				continue
			}
			index[key] = len(result)
			result = append(result, item)
		}
	}
	appendItems(left)
	appendItems(right)
	if len(result) == 0 {
		return nil
	}
	return result
}
