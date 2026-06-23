package usersettings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	domainusersettings "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/usersettings"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

// ErrValidation 表示用户输入校验失败，可被 handler 识别并返回 400。
type ErrValidation struct {
	Msg string
}

func (e *ErrValidation) Error() string { return e.Msg }

// allowedKeys 是用户可配置的 key 集合及其默认值。
var allowedKeys = map[string]string{
	"chat.file_mode":                            "auto",
	"chat.send_on_enter":                        "enter",
	"chat.show_token_usage":                     "true",
	"chat.show_model_info":                      "true",
	"chat.show_latency":                         "true",
	"chat.show_billing_cost":                    "true",
	"chat.default_model":                        "",
	"chat.auto_generate_title":                  "true",
	"chat.delete_conversation_files_by_default": "false",
	"chat.context_compact_auto":                 "true",
	"chat.markdown_render":                      "true",
	"chat.restore_draft_on_failure":             "true",
	"chat.preserve_conversation_drafts":         "true",
	"chat.reuse_model_options":                  "true",
	"chat.input_height":                         "standard",
	"chat.content_width":                        "compact",
	"chat.default_mcp_tool_ids":                 "[]",
}

// boolKeys 取值只能是 "true" / "false"。
var boolKeys = map[string]bool{
	"chat.show_token_usage":                     true,
	"chat.show_model_info":                      true,
	"chat.show_latency":                         true,
	"chat.show_billing_cost":                    true,
	"chat.auto_generate_title":                  true,
	"chat.delete_conversation_files_by_default": true,
	"chat.context_compact_auto":                 true,
	"chat.markdown_render":                      true,
	"chat.restore_draft_on_failure":             true,
	"chat.preserve_conversation_drafts":         true,
	"chat.reuse_model_options":                  true,
}

// enumKeys 枚举 key 的合法值集合。
var enumKeys = map[string]map[string]bool{
	"chat.file_mode":     {"auto": true, "full_context": true, "rag": true},
	"chat.send_on_enter": {"enter": true, "ctrl_enter": true, "meta_enter": true},
	"chat.input_height":  {"compact": true, "standard": true, "loose": true},
	"chat.content_width": {"compact": true, "standard": true, "wide": true},
}

// validateValue 校验 key 对应 value 的合法性。
func validateValue(key, value string) error {
	if key == "chat.default_mcp_tool_ids" {
		return validateDefaultMCPToolIDs(value, key)
	}
	if boolKeys[key] {
		if value != "true" && value != "false" {
			return &ErrValidation{Msg: fmt.Sprintf("invalid value for %s: must be 'true' or 'false'", key)}
		}
	}
	if allowed, ok := enumKeys[key]; ok {
		if !allowed[value] {
			valid := make([]string, 0, len(allowed))
			for v := range allowed {
				valid = append(valid, "'"+v+"'")
			}
			return &ErrValidation{Msg: fmt.Sprintf("invalid value for %s: must be one of %s", key, strings.Join(valid, ", "))}
		}
	}
	return nil
}

func validateDefaultMCPToolIDs(value string, key string) error {
	var toolIDs []uint64
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &toolIDs); err != nil {
		return &ErrValidation{Msg: fmt.Sprintf("invalid value for %s: must be a JSON array of positive tool IDs", key)}
	}
	if len(toolIDs) > 128 {
		return &ErrValidation{Msg: fmt.Sprintf("invalid value for %s: must contain at most 128 tool IDs", key)}
	}
	for _, id := range toolIDs {
		if id == 0 {
			return &ErrValidation{Msg: fmt.Sprintf("invalid value for %s: tool IDs must be positive integers", key)}
		}
	}
	return nil
}

// IsValidationError 判断 err 是否为校验错误。
func IsValidationError(err error) bool {
	var ve *ErrValidation
	return errors.As(err, &ve)
}

// Service 封装用户配置业务逻辑。
type Service struct {
	repo repository.UserSettingsRepository
}

// NewService 创建服务。
func NewService(repo repository.UserSettingsRepository) *Service {
	return &Service{repo: repo}
}

// ListSettings 返回指定用户的全部配置，缺失的 key 用默认值填充。
func (s *Service) ListSettings(ctx context.Context, userID uint) (map[string]string, error) {
	rows, err := s.repo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(allowedKeys))
	// 填充默认值
	for k, v := range allowedKeys {
		result[k] = v
	}
	// 覆盖用户设置值
	for _, row := range rows {
		if _, ok := allowedKeys[row.Key]; ok {
			result[row.Key] = row.Value
		}
	}
	return result, nil
}

// PatchSettings 批量更新用户配置项，返回更新后的全量配置。
func (s *Service) PatchSettings(ctx context.Context, userID uint, patches map[string]string) (map[string]string, error) {
	now := time.Now()
	items := make([]domainusersettings.UserSetting, 0, len(patches))
	for key, value := range patches {
		key = strings.TrimSpace(key)
		if _, ok := allowedKeys[key]; !ok {
			return nil, &ErrValidation{Msg: fmt.Sprintf("unknown setting key: %s", key)}
		}
		if err := validateValue(key, value); err != nil {
			return nil, err
		}
		items = append(items, domainusersettings.UserSetting{
			UserID:    userID,
			Key:       key,
			Value:     value,
			UpdatedAt: now,
		})
	}
	if err := s.repo.Upsert(ctx, items); err != nil {
		return nil, err
	}
	return s.ListSettings(ctx, userID)
}
