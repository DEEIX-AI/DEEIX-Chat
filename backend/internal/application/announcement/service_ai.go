package announcement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

var ErrAnnouncementAIGenerationUnavailable = errors.New("announcement ai generation unavailable")

type GenerateDraftInput struct {
	Requirement string
	Locale      string
}

type GenerateDraftResult struct {
	Title           string `json:"title"`
	ContentMarkdown string `json:"contentMarkdown"`
	Type            string `json:"type"`
	Pinned          bool   `json:"pinned"`
	Priority        int    `json:"priority"`
}

func (s *Service) GenerateDraft(ctx context.Context, input GenerateDraftInput) (*GenerateDraftResult, error) {
	requirement := strings.TrimSpace(input.Requirement)
	if requirement == "" || len([]rune(requirement)) > 4000 {
		return nil, ErrInvalidAnnouncement
	}
	if s == nil || s.routeResolver == nil || s.llmClient == nil {
		return nil, ErrAnnouncementAIGenerationUnavailable
	}

	route, err := s.routeResolver.ResolveDefaultRoute(ctx, channel.ResolveRouteInput{
		TaskType: channel.TaskTypeChat,
		Scope:    channel.RouteScopeInternal,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAnnouncementAIGenerationUnavailable, err)
	}

	systemPrompt := announcementDraftSystemPrompt(input.Locale)
	userPrompt := fmt.Sprintf("管理员需求：\n%s", requirement)
	generateInput := llm.GenerateInput{
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		DisableTools: true,
	}
	routeConfig := s.aiRouteConfig(route)
	out, err := s.llmClient.Generate(ctx, routeConfig, generateInput)
	if err != nil {
		s.routeResolver.MarkRouteFailure(ctx, route, err)
		return nil, fmt.Errorf("generate announcement draft: %w", err)
	}
	s.routeResolver.MarkRouteSuccess(ctx, route)

	result, err := parseAnnouncementDraft(out.Text)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) aiRouteConfig(route *channel.ResolvedRoute) llm.RouteConfig {
	var attributionReferer string
	var attributionTitle string
	if s.cfg != nil {
		cfg := s.cfg.Snapshot()
		attributionReferer = cfg.PublicWebBaseURL
		attributionTitle = cfg.AppName
	}
	return llm.RouteConfig{
		Protocol:            route.Protocol,
		BaseURL:             route.BaseURL,
		APIKey:              route.APIKey,
		HeadersJSON:         route.HeadersJSON,
		ConnectTimeoutMS:    route.ConnectTimeoutMS,
		ReadTimeoutMS:       route.ReadTimeoutMS,
		StreamIdleTimeoutMS: route.StreamIdleTimeoutMS,
		Endpoint:            llm.DefaultEndpointForAdapter(route.Protocol),
		UpstreamModel:       route.UpstreamModel,
		AttributionReferer:  attributionReferer,
		AttributionTitle:    attributionTitle,
	}
}

func announcementDraftSystemPrompt(locale string) string {
	language := "中文"
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "en") {
		language = "English"
	}
	return fmt.Sprintf(`你是 DEEIX Chat 的产品运营公告编辑。请根据管理员需求生成一条可直接发布的站内公告，语言使用%s。

只输出一个 JSON 对象，不要 Markdown 代码块，不要解释。字段：
- title: 12-36 字，清晰具体，不夸张。
- contentMarkdown: 美观 Markdown，适合站内弹窗阅读；包含短导语、2-4 个要点列表，必要时包含注意事项；不要使用 HTML；不要虚构日期、版本号、价格、链接或承诺。
- type: general | normal | info | warning | critical。普通通知用 general，功能更新用 info，成功/完成用 normal，影响使用但不紧急用 warning，严重中断或安全风险用 critical。
- pinned: boolean，重要或有时效影响时为 true。
- priority: integer，普通 0-20，重要 30-60，紧急 80-100。

内容要简洁、友好、可信，排版有层次。`, language)
}

func parseAnnouncementDraft(raw string) (*GenerateDraftResult, error) {
	objectText := extractJSONObject(raw)
	if objectText == "" {
		return nil, ErrInvalidAnnouncement
	}
	var draft GenerateDraftResult
	if err := json.Unmarshal([]byte(objectText), &draft); err != nil {
		return nil, ErrInvalidAnnouncement
	}
	draft.Title = strings.TrimSpace(draft.Title)
	draft.ContentMarkdown = strings.TrimSpace(draft.ContentMarkdown)
	draft.Type = normalizeType(draft.Type)
	if draft.Title == "" || len(draft.Title) > maxAnnouncementTitleLength || draft.ContentMarkdown == "" {
		return nil, ErrInvalidAnnouncement
	}
	if len(draft.ContentMarkdown) > maxAnnouncementContentLength {
		draft.ContentMarkdown = truncateUTF8Bytes(draft.ContentMarkdown, maxAnnouncementContentLength)
	}
	if draft.Type == "" {
		draft.Type = "general"
	}
	if draft.Priority < 0 {
		draft.Priority = 0
	}
	if draft.Priority > 100 {
		draft.Priority = 100
	}
	return &draft, nil
}

func truncateUTF8Bytes(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		if maxBytes <= 0 {
			return ""
		}
		return value
	}
	for index := range value {
		if index > maxBytes {
			return strings.TrimSpace(value[:index])
		}
	}
	return strings.TrimSpace(value)
}

func extractJSONObject(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "```") {
		text = strings.TrimSpace(strings.Trim(text, "`"))
		if idx := strings.Index(text, "\n"); idx >= 0 {
			text = strings.TrimSpace(text[idx+1:])
		}
	}
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return ""
	}
	return text[start : end+1]
}
