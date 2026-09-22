package llm

import (
	"context"
	"net/url"
	"strings"

	portllm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// listModelsRequesty 拉取 Requesty 模型目录。
// Requesty 走 OpenAI 兼容协议，但除通用 /models（vendor/model 形式的完整目录）外，
// 还提供 /models/managed：由 Requesty 维护的多提供商路由策略，id 为短稳定名（如 claude-sonnet-4-5），
// 可直接作为 model 使用。两份目录响应形状相同，这里先列托管策略，再补齐完整目录并去重。
// 完整目录需要有效密钥（无效密钥返回 403），托管策略目录公开可读；
// 托管目录不可用时退回仅返回完整目录，避免阻断模型同步。
func (c *Client) listModelsRequesty(ctx context.Context, route portllm.RouteConfig) ([]portllm.ModelItem, error) {
	catalog, err := c.listModelsFromURL(ctx, route, buildOpenAIModelsURL(route.BaseURL))
	if err != nil {
		return nil, err
	}
	managed, err := c.listModelsFromURL(ctx, route, buildRequestyManagedModelsURL(route.BaseURL))
	if err != nil {
		return catalog, nil
	}
	return mergeModelItems(managed, catalog), nil
}

func buildRequestyManagedModelsURL(baseURL string) string {
	return buildVersionedEndpointURL(baseURL, "v1", "/models/managed")
}

// mergeModelItems 按顺序合并多份模型目录，同 id 只保留首次出现。
func mergeModelItems(lists ...[]portllm.ModelItem) []portllm.ModelItem {
	total := 0
	for _, list := range lists {
		total += len(list)
	}
	merged := make([]portllm.ModelItem, 0, total)
	seen := make(map[string]struct{}, total)
	for _, list := range lists {
		for _, item := range list {
			if _, ok := seen[item.ID]; ok {
				continue
			}
			seen[item.ID] = struct{}{}
			merged = append(merged, item)
		}
	}
	return merged
}

func isRequestyBaseURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return host == "requesty.ai" || strings.HasSuffix(host, ".requesty.ai")
}
