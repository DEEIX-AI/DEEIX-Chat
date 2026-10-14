package conversation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	// maxUIStateBlockIDChars 是组件块 ID 的长度上限（字节），与仓储层索引列宽一致。
	maxUIStateBlockIDChars = 64
	// maxUIStateKeyChars 是状态名的长度上限（字节）。
	maxUIStateKeyChars = 64
	// maxUIStateStateBytes 是单个块状态快照的字节上限。
	maxUIStateStateBytes = 4 << 10
	// maxUIStateDepth 是状态快照单个值允许的最大嵌套深度（标量记为第 1 层）。
	// 内置组件的状态最多两层（如测验的 answers 数组），4 层留有余量，同时挡住深层嵌套带来的解析开销。
	maxUIStateDepth = 4
	// maxUIStateBlocksPerMessage 是单条消息一次可写入的组件块数量上限；
	// 与仓储层 maxUIStateBlocksPerMessage 取值一致，仓储层在事务内复核总块数。
	maxUIStateBlocksPerMessage = 32
)

var (
	// uiStateNamePattern 约束组件块 ID 与状态名：与提示词要求的“简短英文 id”一致，
	// 也保证写进上下文 XML 的名字可读、不含需要转义的字符。长度另行按字节校验。
	uiStateNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// MessageUIStateBlockPatch 描述单个组件块的本次状态变更；StateJSON 为空表示清除该块状态。
type MessageUIStateBlockPatch struct {
	BlockID   string
	StateJSON string
}

// MessageUIStateResult 返回写入后的消息组件状态快照。
type MessageUIStateResult struct {
	MessageID       uint
	MessagePublicID string
	// Blocks 是本次写入后该消息的全部块状态，前端据此校正本地缓存。
	Blocks map[string]json.RawMessage
}

// SetMessageUIState 写入或清除某条助手消息内交互式组件的状态快照。
// 只接受与块初始值不同的取值：客户端把恢复所需的"原值"留在块定义里，服务端只存差量。
func (s *Service) SetMessageUIState(
	ctx context.Context,
	userID uint,
	conversationPublicID string,
	messagePublicID string,
	patches []MessageUIStateBlockPatch,
) (*MessageUIStateResult, error) {
	if userID == 0 || len(patches) == 0 {
		return nil, ErrInvalidMessageUIState
	}

	normalizedConversationID := strings.TrimSpace(conversationPublicID)
	normalizedMessageID := strings.TrimSpace(messagePublicID)
	if normalizedConversationID == "" {
		return nil, ErrConversationNotFound
	}
	if normalizedMessageID == "" {
		return nil, ErrMessageNotFound
	}
	if len(patches) > maxUIStateBlocksPerMessage {
		return nil, ErrInvalidMessageUIState
	}

	conversation, err := s.repo.GetConversationByPublicID(ctx, normalizedConversationID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrConversationNotFound
		}
		return nil, err
	}

	message, err := s.repo.GetMessageByPublicIDForUser(ctx, userID, normalizedMessageID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrMessageNotFound
		}
		return nil, err
	}
	if message.ConversationID != conversation.ID {
		return nil, ErrMessageNotFound
	}
	if message.Role != "assistant" {
		return nil, ErrMessageUIStateTargetInvalid
	}
	// 生成中的消息仍在流式追加块，此时保存的状态会指向还没写完的块，直接拒绝。
	if strings.EqualFold(strings.TrimSpace(message.Status), "pending") {
		return nil, ErrMessageUIStateTargetInvalid
	}

	blocks := make(map[string]string, len(patches))
	seenBlockIDs := make(map[string]struct{}, len(patches))
	for _, patch := range patches {
		blockID := strings.TrimSpace(patch.BlockID)
		if !validUIStateBlockID(blockID) {
			return nil, ErrInvalidMessageUIState
		}
		if _, duplicated := seenBlockIDs[blockID]; duplicated {
			return nil, ErrInvalidMessageUIState
		}
		seenBlockIDs[blockID] = struct{}{}

		normalizedState, err := normalizeUIStateJSON(patch.StateJSON)
		if err != nil {
			return nil, err
		}
		blocks[blockID] = normalizedState
	}

	if err := s.repo.UpsertMessageUIStates(ctx, userID, conversation.ID, message.ID, blocks); err != nil {
		switch {
		case errors.Is(err, repository.ErrMessageUIStateLimitExceeded), errors.Is(err, repository.ErrMessageUIStateInvalid):
			return nil, ErrInvalidMessageUIState
		case errors.Is(err, repository.ErrNotFound):
			// 预检之后消息被删除：仓储层持锁复核时发现消息已不存在。
			return nil, ErrMessageNotFound
		default:
			return nil, err
		}
	}

	stateMap, err := s.repo.GetMessageUIStateMap(ctx, userID, []uint{message.ID})
	if err != nil {
		return nil, err
	}
	stored := stateMap[message.ID]
	result := &MessageUIStateResult{
		MessageID:       message.ID,
		MessagePublicID: message.PublicID,
		Blocks:          make(map[string]json.RawMessage, len(stored)),
	}
	for blockID, stateJSON := range stored {
		result.Blocks[blockID] = json.RawMessage(stateJSON)
	}
	return result, nil
}

// validUIStateBlockID 判断组件块 ID 是否可用于保存与注入。
func validUIStateBlockID(blockID string) bool {
	return len(blockID) <= maxUIStateBlockIDChars && uiStateNamePattern.MatchString(blockID)
}

// normalizeUIStateJSON 校验并规范化块状态快照；空对象视为清除，返回空串。
func normalizeUIStateJSON(raw string) (string, error) {
	text := strings.TrimSpace(raw)
	if text == "" || text == "{}" {
		return "", nil
	}
	if len(text) > maxUIStateStateBytes {
		return "", ErrInvalidMessageUIState
	}

	var payload map[string]any
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return "", ErrInvalidMessageUIState
	}
	if decoder.More() {
		return "", ErrInvalidMessageUIState
	}
	if payload == nil {
		return "", ErrInvalidMessageUIState
	}
	if len(payload) == 0 {
		return "", nil
	}
	for key, value := range payload {
		if len(key) > maxUIStateKeyChars || !uiStateNamePattern.MatchString(key) || jsonDepth(value, 1) > maxUIStateDepth {
			return "", ErrInvalidMessageUIState
		}
	}

	// 关闭 HTML 转义：状态会原样进入提示词（注入时另做 XML 转义），\u003c 之类的转义只会降低可读性，
	// 也让入库大小与前端按 JSON.stringify 估算的大小保持一致。
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(payload); err != nil {
		return "", ErrInvalidMessageUIState
	}
	encoded := bytes.TrimRight(buffer.Bytes(), "\n")
	if len(encoded) > maxUIStateStateBytes {
		return "", ErrInvalidMessageUIState
	}
	return string(encoded), nil
}

// jsonDepth 计算 JSON 值的嵌套深度，用于拒绝结构过深的状态快照。
func jsonDepth(value any, depth int) int {
	switch typed := value.(type) {
	case map[string]any:
		maxChild := depth
		for _, child := range typed {
			if childDepth := jsonDepth(child, depth+1); childDepth > maxChild {
				maxChild = childDepth
			}
		}
		return maxChild
	case []any:
		maxChild := depth
		for _, child := range typed {
			if childDepth := jsonDepth(child, depth+1); childDepth > maxChild {
				maxChild = childDepth
			}
		}
		return maxChild
	default:
		return depth
	}
}

// hydrateMessageUIStates 为所有者读取路径填充消息的组件状态快照。
func (s *Service) hydrateMessageUIStates(ctx context.Context, userID uint, items []model.Message) error {
	if len(items) == 0 {
		return nil
	}

	messageIDs := make([]uint, 0, len(items))
	for _, item := range items {
		if item.ID == 0 || item.Role != "assistant" {
			continue
		}
		messageIDs = append(messageIDs, item.ID)
	}
	if len(messageIDs) == 0 {
		return nil
	}

	stateMap, err := s.repo.GetMessageUIStateMap(ctx, userID, messageIDs)
	if err != nil {
		return err
	}
	for i := range items {
		if items[i].Role != "assistant" {
			continue
		}
		if blocks, ok := stateMap[items[i].ID]; ok && len(blocks) > 0 {
			items[i].UIState = blocks
		}
	}
	return nil
}

// MergedMessageUIState 把块状态合并为一个 JSON 对象，供响应 DTO 直接序列化。
func MergedMessageUIState(blocks map[string]string) map[string]json.RawMessage {
	if len(blocks) == 0 {
		return nil
	}
	merged := make(map[string]json.RawMessage, len(blocks))
	for blockID, stateJSON := range blocks {
		text := strings.TrimSpace(stateJSON)
		if text == "" || !json.Valid([]byte(text)) {
			continue
		}
		// 规范化一次，保证响应体里的 JSON 与写入时一致（紧凑、无多余空白）。
		var compact bytes.Buffer
		if err := json.Compact(&compact, []byte(text)); err != nil {
			continue
		}
		merged[blockID] = json.RawMessage(compact.String())
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}
