package conversation

import "strings"

// CollectMessageRunIDs 提取消息关联的运行 ID，并保持首次出现顺序。
func CollectMessageRunIDs(messages []Message) []string {
	seen := make(map[string]struct{}, len(messages))
	runIDs := make([]string, 0, len(messages))
	for _, message := range messages {
		runID := strings.TrimSpace(message.RunID)
		if runID == "" {
			continue
		}
		if _, exists := seen[runID]; exists {
			continue
		}
		seen[runID] = struct{}{}
		runIDs = append(runIDs, runID)
	}
	return runIDs
}
