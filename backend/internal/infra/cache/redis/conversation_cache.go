package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/go-redis/redis/v8"
)

// ragCachePayload 是 RAG 缓存的序列化格式，仅限 infra 层使用。
type ragCachePayload struct {
	Chunks []ragCacheChunk `json:"chunks"`
}

// ragCacheChunk RAG 缓存中单个文本片段的序列化格式。
type ragCacheChunk struct {
	Content    string  `json:"content"`
	FileName   string  `json:"file_name"`
	FileID     string  `json:"file_id"`
	ChunkIndex int     `json:"chunk_index"`
	Score      float32 `json:"score"`
}

const (
	fileProcessingStreamName = "file_processing_v1"
	fileProcessingDLQName    = "file_processing_v1_dlq"
	fileProcessingGroupName  = "file_processing_workers"
	fileProcessingMinIdle    = 45 * time.Second

	generationStreamKeyPrefix = "conversation:generation:"
)

// conversationCache 实现 repository.ConversationCacheRepository。
type conversationCache struct {
	client *redis.Client
}

// NewConversationCache 创建 ConversationCacheRepository 实现。
func NewConversationCache(client *redis.Client) repository.ConversationCacheRepository {
	return &conversationCache{client: client}
}

// ---------------------------------------------------------------------------
// 文件处理队列
// ---------------------------------------------------------------------------

// InitFileProcessingStream 初始化文件处理 Redis Stream 及消费者组，幂等。
func (c *conversationCache) InitFileProcessingStream(ctx context.Context) error {
	if c.client == nil {
		return nil
	}
	err := c.client.XGroupCreateMkStream(ctx, fileProcessingStreamName, fileProcessingGroupName, "0").Err()
	if err != nil && strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return err
}

// EnqueueFileProcessing 将文件处理任务推入 Stream 队列。
func (c *conversationCache) EnqueueFileProcessing(ctx context.Context, userID uint, fileID string, retry int, lastError string) error {
	if c.client == nil {
		return nil
	}
	values := map[string]interface{}{
		"user_id": userID,
		"file_id": fileID,
		"retry":   retry,
	}
	if strings.TrimSpace(lastError) != "" {
		values["last_error"] = truncateStr(lastError, 255)
	}
	_, err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: fileProcessingStreamName,
		Values: values,
	}).Result()
	return err
}

// ClaimTimedOutFileProcessingMessages 认领超时未确认的 pending 任务，避免 worker 重启后任务永久卡住。
func (c *conversationCache) ClaimTimedOutFileProcessingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
	if c.client == nil {
		return nil, nil
	}
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: fileProcessingStreamName,
		Group:  fileProcessingGroupName,
		Idle:   fileProcessingMinIdle,
		Start:  "-",
		End:    "+",
		Count:  1,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	if len(pending) == 0 {
		return nil, nil
	}
	messageIDs := make([]string, 0, len(pending))
	for _, item := range pending {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		messageIDs = append(messageIDs, item.ID)
	}
	if len(messageIDs) == 0 {
		return nil, nil
	}
	claimed, err := c.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   fileProcessingStreamName,
		Group:    fileProcessingGroupName,
		Consumer: consumerName,
		MinIdle:  fileProcessingMinIdle,
		Messages: messageIDs,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	messages := make([]repository.FileProcessingMessage, 0, len(claimed))
	for _, msg := range claimed {
		messages = append(messages, parseFileProcessingMessage(msg))
	}
	return messages, nil
}

// ReadFileProcessingMessages 阻塞读取未处理消息（最多 1 条，5s 超时）。
func (c *conversationCache) ReadFileProcessingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
	if c.client == nil {
		return nil, nil
	}
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    fileProcessingGroupName,
		Consumer: consumerName,
		Streams:  []string{fileProcessingStreamName, ">"},
		Count:    1,
		Block:    5 * time.Second,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	messages := make([]repository.FileProcessingMessage, 0)
	for _, stream := range streams {
		for _, msg := range stream.Messages {
			messages = append(messages, parseFileProcessingMessage(msg))
		}
	}
	return messages, nil
}

func parseFileProcessingMessage(msg redis.XMessage) repository.FileProcessingMessage {
	return repository.FileProcessingMessage{
		ID:        msg.ID,
		UserID:    uint(getInt64Val(msg.Values["user_id"])),
		FileID:    strings.TrimSpace(getStringVal(msg.Values["file_id"])),
		Retry:     int(getInt64Val(msg.Values["retry"])),
		LastError: getStringVal(msg.Values["last_error"]),
	}
}

// AckFileProcessingMessage 确认消息已处理。
func (c *conversationCache) AckFileProcessingMessage(ctx context.Context, messageID string) error {
	if c.client == nil {
		return nil
	}
	_, err := c.client.XAck(ctx, fileProcessingStreamName, fileProcessingGroupName, messageID).Result()
	return err
}

// DeleteFileProcessingMessage 从 Stream 中删除消息。
func (c *conversationCache) DeleteFileProcessingMessage(ctx context.Context, messageID string) error {
	if c.client == nil {
		return nil
	}
	_, err := c.client.XDel(ctx, fileProcessingStreamName, messageID).Result()
	return err
}

// SendFileProcessingToDLQ 将超过重试次数的消息写入死信队列。
func (c *conversationCache) SendFileProcessingToDLQ(ctx context.Context, userID uint, fileID string, retry int, lastError string) error {
	if c.client == nil {
		return nil
	}
	_, err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: fileProcessingDLQName,
		Values: map[string]interface{}{
			"user_id":    userID,
			"file_id":    fileID,
			"retry":      retry,
			"last_error": truncateStr(lastError, 255),
		},
	}).Result()
	return err
}

// ---------------------------------------------------------------------------
// RAG 缓存
// ---------------------------------------------------------------------------

// GetRAGCache 读取 RAG 检索缓存，未命中时 ok=false。
func (c *conversationCache) GetRAGCache(ctx context.Context, key string) ([]domainconversation.RAGChunk, bool) {
	if c.client == nil {
		return nil, false
	}
	raw, err := c.client.Get(ctx, key).Result()
	if err != nil {
		return nil, false
	}
	var payload ragCachePayload
	if err = json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, false
	}
	chunks := make([]domainconversation.RAGChunk, 0, len(payload.Chunks))
	for _, c := range payload.Chunks {
		chunks = append(chunks, domainconversation.RAGChunk{
			Content:    c.Content,
			FileName:   c.FileName,
			FileID:     c.FileID,
			ChunkIndex: c.ChunkIndex,
			Score:      c.Score,
		})
	}
	return chunks, true
}

// SetRAGCache 写入 RAG 检索缓存。
func (c *conversationCache) SetRAGCache(ctx context.Context, key string, chunks []domainconversation.RAGChunk, ttl time.Duration) {
	if c.client == nil {
		return
	}
	rawChunks := make([]ragCacheChunk, 0, len(chunks))
	for _, ch := range chunks {
		rawChunks = append(rawChunks, ragCacheChunk{
			Content:    ch.Content,
			FileName:   ch.FileName,
			FileID:     ch.FileID,
			ChunkIndex: ch.ChunkIndex,
			Score:      ch.Score,
		})
	}
	data, err := json.Marshal(ragCachePayload{Chunks: rawChunks})
	if err != nil {
		return
	}
	_ = c.client.Set(ctx, key, data, ttl).Err()
}

// ---------------------------------------------------------------------------
// 生成流恢复
// ---------------------------------------------------------------------------

// RegisterGenerationStream 记录 run 归属用户，并清除上一轮取消标记。
func (c *conversationCache) RegisterGenerationStream(ctx context.Context, runID string, userID uint, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	pipe := c.client.Pipeline()
	pipe.Set(ctx, generationStreamOwnerKey(runID), strconv.FormatUint(uint64(userID), 10), ttl)
	pipe.Del(ctx, generationStreamCancelKey(runID))
	_, err := pipe.Exec(ctx)
	return err
}

// GetGenerationStreamOwner 返回 run 归属用户。
func (c *conversationCache) GetGenerationStreamOwner(ctx context.Context, runID string) (uint, bool, error) {
	if c.client == nil {
		return 0, false, nil
	}
	raw, err := c.client.Get(ctx, generationStreamOwnerKey(runID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, false, nil
		}
		return 0, false, err
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value <= 0 {
		return 0, false, nil
	}
	return uint(value), true, nil
}

// TouchGenerationStreamActive 刷新 run 的活跃租约。
func (c *conversationCache) TouchGenerationStreamActive(ctx context.Context, runID string, ttl time.Duration) error {
	if c.client == nil || ttl <= 0 {
		return nil
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	return c.client.Set(ctx, generationStreamActiveKey(runID), "1", ttl).Err()
}

// ClearGenerationStreamActive 清理 run 的活跃租约。
func (c *conversationCache) ClearGenerationStreamActive(ctx context.Context, runID string) error {
	if c.client == nil {
		return nil
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	return c.client.Del(ctx, generationStreamActiveKey(runID)).Err()
}

// IsGenerationStreamActive 查询 run 是否仍有活跃生成租约。
func (c *conversationCache) IsGenerationStreamActive(ctx context.Context, runID string) (bool, error) {
	if c.client == nil {
		return false, nil
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return false, nil
	}
	count, err := c.client.Exists(ctx, generationStreamActiveKey(runID)).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RequestGenerationStreamCancel 标记 run 已被用户显式取消。
func (c *conversationCache) RequestGenerationStreamCancel(ctx context.Context, runID string, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}
	return c.client.Set(ctx, generationStreamCancelKey(runID), "1", ttl).Err()
}

// IsGenerationStreamCanceled 查询 run 是否已被显式取消。
func (c *conversationCache) IsGenerationStreamCanceled(ctx context.Context, runID string) (bool, error) {
	if c.client == nil {
		return false, nil
	}
	count, err := c.client.Exists(ctx, generationStreamCancelKey(runID)).Result()
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// AppendGenerationStreamEvent 追加生成流事件，使用独立 seq 保持前端游标稳定。
func (c *conversationCache) AppendGenerationStreamEvent(ctx context.Context, runID string, payloadJSON string, maxEvents int64, ttl time.Duration) (repository.GenerationStreamMessage, error) {
	if c.client == nil {
		return repository.GenerationStreamMessage{}, nil
	}
	if maxEvents <= 0 {
		maxEvents = 1024
	}
	seq, err := c.client.Incr(ctx, generationStreamSeqKey(runID)).Result()
	if err != nil {
		return repository.GenerationStreamMessage{}, err
	}
	id, err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream:       generationStreamEventsKey(runID),
		MaxLenApprox: maxEvents,
		Values: map[string]interface{}{
			"seq":     seq,
			"payload": payloadJSON,
		},
	}).Result()
	if err != nil {
		return repository.GenerationStreamMessage{}, err
	}
	_ = c.ExpireGenerationStream(ctx, runID, ttl)
	return repository.GenerationStreamMessage{ID: id, Seq: seq, PayloadJSON: payloadJSON}, nil
}

// ListGenerationStreamEvents 返回当前保留窗口内的生成流事件。
func (c *conversationCache) ListGenerationStreamEvents(ctx context.Context, runID string, limit int64) ([]repository.GenerationStreamMessage, error) {
	if c.client == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 1024
	}
	items, err := c.client.XRevRangeN(ctx, generationStreamEventsKey(runID), "+", "-", limit).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	for left, right := 0, len(items)-1; left < right; left, right = left+1, right-1 {
		items[left], items[right] = items[right], items[left]
	}
	return parseGenerationStreamMessages(items), nil
}

// ReadGenerationStreamEvents 阻塞读取 afterID 之后的生成流事件。
func (c *conversationCache) ReadGenerationStreamEvents(ctx context.Context, runID string, afterID string, block time.Duration, limit int64) ([]repository.GenerationStreamMessage, error) {
	if c.client == nil {
		return nil, nil
	}
	if strings.TrimSpace(afterID) == "" {
		afterID = "0-0"
	}
	if block <= 0 {
		block = 5 * time.Second
	}
	if limit <= 0 {
		limit = 128
	}
	streams, err := c.client.XRead(ctx, &redis.XReadArgs{
		Streams: []string{generationStreamEventsKey(runID), afterID},
		Count:   limit,
		Block:   block,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	results := make([]repository.GenerationStreamMessage, 0)
	for _, stream := range streams {
		results = append(results, parseGenerationStreamMessages(stream.Messages)...)
	}
	return results, nil
}

// ExpireGenerationStream 设置生成流相关键的过期时间。
func (c *conversationCache) ExpireGenerationStream(ctx context.Context, runID string, ttl time.Duration) error {
	if c.client == nil || ttl <= 0 {
		return nil
	}
	pipe := c.client.Pipeline()
	pipe.Expire(ctx, generationStreamEventsKey(runID), ttl)
	pipe.Expire(ctx, generationStreamSeqKey(runID), ttl)
	pipe.Expire(ctx, generationStreamOwnerKey(runID), ttl)
	pipe.Expire(ctx, generationStreamCancelKey(runID), ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func parseGenerationStreamMessages(items []redis.XMessage) []repository.GenerationStreamMessage {
	results := make([]repository.GenerationStreamMessage, 0, len(items))
	for _, item := range items {
		payload := strings.TrimSpace(getStringVal(item.Values["payload"]))
		if payload == "" {
			continue
		}
		results = append(results, repository.GenerationStreamMessage{
			ID:          item.ID,
			Seq:         getInt64Val(item.Values["seq"]),
			PayloadJSON: payload,
		})
	}
	return results
}

// ---------------------------------------------------------------------------
// 内部辅助
// ---------------------------------------------------------------------------

func generationStreamEventsKey(runID string) string {
	return generationStreamKeyPrefix + strings.TrimSpace(runID) + ":events"
}

func generationStreamSeqKey(runID string) string {
	return generationStreamKeyPrefix + strings.TrimSpace(runID) + ":seq"
}

func generationStreamOwnerKey(runID string) string {
	return generationStreamKeyPrefix + strings.TrimSpace(runID) + ":owner"
}

func generationStreamActiveKey(runID string) string {
	return generationStreamKeyPrefix + strings.TrimSpace(runID) + ":active"
}

func generationStreamCancelKey(runID string) string {
	return generationStreamKeyPrefix + strings.TrimSpace(runID) + ":cancel"
}

func truncateStr(s string, maxLen int) string {
	v := strings.TrimSpace(s)
	if maxLen <= 0 || len([]rune(v)) <= maxLen {
		return v
	}
	return string([]rune(v)[:maxLen])
}

func getStringVal(raw interface{}) string {
	switch v := raw.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", raw)
	}
}

func getInt64Val(raw interface{}) int64 {
	switch v := raw.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	case []byte:
		n, _ := strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64)
		return n
	default:
		return 0
	}
}
