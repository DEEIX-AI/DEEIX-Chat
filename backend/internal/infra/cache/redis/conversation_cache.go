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
	fileProcessingDLQMaxLen  = 10_000

	generationStreamKeyPrefix = "conversation:generation:"
	generationStreamIndexTTL  = 2 * time.Hour
)

// appendGenerationStreamEventScript keeps the event sequence, bounded replay
// window, and cumulative visible-text checkpoint consistent in one Redis
// round trip. Key TTLs are initialized only when a value is first created;
// FinishMessageGeneration shortens them to the post-run retention window.
var appendGenerationStreamEventScript = redis.NewScript(`
local events_missing = redis.call("EXISTS", KEYS[2]) == 0
local has_text_delta = ARGV[4] ~= ""
local text_missing = false
if has_text_delta then
	text_missing = redis.call("EXISTS", KEYS[3]) == 0
end
local seq = redis.call("INCR", KEYS[1])
if has_text_delta then
	redis.call("APPEND", KEYS[3], ARGV[4])
	redis.call("SET", KEYS[4], tostring(seq), "KEEPTTL")
end
local id = redis.call(
	"XADD",
	KEYS[2],
	"MAXLEN", "~", ARGV[2],
	"*",
	"seq", tostring(seq),
	"payload", ARGV[1]
)

if seq == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[3])
end
if events_missing then
	redis.call("PEXPIRE", KEYS[2], ARGV[3])
end
if has_text_delta and text_missing then
	redis.call("PEXPIRE", KEYS[3], ARGV[3])
	redis.call("PEXPIRE", KEYS[4], ARGV[3])
end

return {id, tostring(seq)}
`)

var renewFileProcessingLeaseScript = redis.NewScript(`
local pending = redis.call("XPENDING", KEYS[1], ARGV[1], ARGV[3], ARGV[3], 1)
if #pending == 0 or pending[1][2] ~= ARGV[2] then
	return 0
end
redis.call("XCLAIM", KEYS[1], ARGV[1], ARGV[2], 0, ARGV[3], "JUSTID")
return 1
`)

var settleFileProcessingMessageScript = redis.NewScript(`
local pending = redis.call("XPENDING", KEYS[1], ARGV[1], ARGV[3], ARGV[3], 1)
if #pending == 0 or pending[1][2] ~= ARGV[2] then
	return 0
end
redis.call("XACK", KEYS[1], ARGV[1], ARGV[3])
redis.call("XDEL", KEYS[1], ARGV[3])
return 1
`)

var requeueFileProcessingMessageScript = redis.NewScript(`
local pending = redis.call("XPENDING", KEYS[1], ARGV[1], ARGV[3], ARGV[3], 1)
if #pending == 0 or pending[1][2] ~= ARGV[2] then
	return 0
end
redis.call(
	"XADD", KEYS[1], "*",
	"user_id", ARGV[4],
	"file_id", ARGV[5],
	"retry", ARGV[6],
	"last_error", ARGV[7]
)
redis.call("XACK", KEYS[1], ARGV[1], ARGV[3])
redis.call("XDEL", KEYS[1], ARGV[3])
return 1
`)

var deadLetterFileProcessingMessageScript = redis.NewScript(`
local pending = redis.call("XPENDING", KEYS[1], ARGV[1], ARGV[3], ARGV[3], 1)
if #pending == 0 or pending[1][2] ~= ARGV[2] then
	return 0
end
redis.call(
	"XADD", KEYS[2], "MAXLEN", ARGV[8], "*",
	"user_id", ARGV[4],
	"file_id", ARGV[5],
	"retry", ARGV[6],
	"last_error", ARGV[7]
)
redis.call("XACK", KEYS[1], ARGV[1], ARGV[3])
redis.call("XDEL", KEYS[1], ARGV[3])
return 1
`)

var touchGenerationStreamActiveScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) ~= ARGV[1] then
	return 0
end
redis.call("SET", KEYS[2], "1", "PX", ARGV[2])
redis.call("ZADD", KEYS[3], ARGV[3], ARGV[4])
redis.call("PEXPIRE", KEYS[3], ARGV[5])
return 1
`)

var clearGenerationStreamActiveScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	redis.call("DEL", KEYS[2])
end
redis.call("ZREM", KEYS[3], ARGV[2])
return 1
`)

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
	return c.decodeFileProcessingMessages(ctx, consumerName, claimed, true)
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
		parsed, parseErr := c.decodeFileProcessingMessages(ctx, consumerName, stream.Messages, false)
		if parseErr != nil {
			return nil, parseErr
		}
		messages = append(messages, parsed...)
	}
	return messages, nil
}

func (c *conversationCache) decodeFileProcessingMessages(
	ctx context.Context,
	consumerName string,
	messages []redis.XMessage,
	reclaimed bool,
) ([]repository.FileProcessingMessage, error) {
	parsedMessages := make([]repository.FileProcessingMessage, 0, len(messages))
	for _, msg := range messages {
		parsed, err := parseFileProcessingMessage(msg)
		if err != nil {
			quarantined, quarantineErr := c.deadLetterInvalidFileProcessingMessage(ctx, consumerName, msg, err)
			if quarantineErr != nil {
				return nil, fmt.Errorf("dead-letter invalid file processing message %q: %w", msg.ID, quarantineErr)
			}
			if !quarantined {
				return nil, fmt.Errorf("dead-letter invalid file processing message %q: message ownership lost", msg.ID)
			}
			continue
		}
		parsed.Reclaimed = reclaimed
		parsedMessages = append(parsedMessages, parsed)
	}
	return parsedMessages, nil
}

func parseFileProcessingMessage(msg redis.XMessage) (repository.FileProcessingMessage, error) {
	userID, err := strconv.ParseUint(strings.TrimSpace(getStringVal(msg.Values["user_id"])), 10, strconv.IntSize)
	if err != nil || userID == 0 {
		if err == nil {
			err = errors.New("must be greater than zero")
		}
		return repository.FileProcessingMessage{}, fmt.Errorf("invalid user_id: %w", err)
	}

	retry, err := strconv.Atoi(strings.TrimSpace(getStringVal(msg.Values["retry"])))
	if err != nil || retry < 0 {
		if err == nil {
			err = errors.New("must not be negative")
		}
		return repository.FileProcessingMessage{}, fmt.Errorf("invalid retry: %w", err)
	}

	lastError := ""
	if rawLastError, ok := msg.Values["last_error"]; ok {
		lastError = getStringVal(rawLastError)
	}

	return repository.FileProcessingMessage{
		ID:        msg.ID,
		UserID:    uint(userID),
		FileID:    strings.TrimSpace(getStringVal(msg.Values["file_id"])),
		Retry:     retry,
		LastError: lastError,
	}, nil
}

func (c *conversationCache) deadLetterInvalidFileProcessingMessage(
	ctx context.Context,
	consumerName string,
	message redis.XMessage,
	parseErr error,
) (bool, error) {
	lastError := "invalid queue message: " + parseErr.Error()
	if rawLastError, ok := message.Values["last_error"]; ok {
		if previousError := strings.TrimSpace(getStringVal(rawLastError)); previousError != "" {
			lastError += "; previous error: " + previousError
		}
	}

	return fileProcessingScriptResult(deadLetterFileProcessingMessageScript.Run(
		ctx,
		c.client,
		[]string{fileProcessingStreamName, fileProcessingDLQName},
		fileProcessingGroupName,
		consumerName,
		message.ID,
		getStringVal(message.Values["user_id"]),
		getStringVal(message.Values["file_id"]),
		getStringVal(message.Values["retry"]),
		truncateStr(lastError, 255),
		fileProcessingDLQMaxLen,
	).Result())
}

// RenewFileProcessingMessageLease 刷新执行中消息的空闲时间，避免长任务被其他 worker 重复认领。
func (c *conversationCache) RenewFileProcessingMessageLease(ctx context.Context, consumerName, messageID string) (bool, error) {
	if c.client == nil || strings.TrimSpace(consumerName) == "" || strings.TrimSpace(messageID) == "" {
		return true, nil
	}
	return fileProcessingScriptResult(renewFileProcessingLeaseScript.Run(
		ctx,
		c.client,
		[]string{fileProcessingStreamName},
		fileProcessingGroupName,
		consumerName,
		messageID,
	).Result())
}

func (c *conversationCache) SettleFileProcessingMessage(ctx context.Context, consumerName, messageID string) (bool, error) {
	if c.client == nil {
		return true, nil
	}
	return fileProcessingScriptResult(settleFileProcessingMessageScript.Run(
		ctx,
		c.client,
		[]string{fileProcessingStreamName},
		fileProcessingGroupName,
		consumerName,
		messageID,
	).Result())
}

func (c *conversationCache) RequeueFileProcessingMessage(
	ctx context.Context,
	consumerName string,
	message repository.FileProcessingMessage,
	retry int,
	lastError string,
) (bool, error) {
	if c.client == nil {
		return true, nil
	}
	return fileProcessingScriptResult(requeueFileProcessingMessageScript.Run(
		ctx,
		c.client,
		[]string{fileProcessingStreamName},
		fileProcessingGroupName,
		consumerName,
		message.ID,
		message.UserID,
		message.FileID,
		retry,
		truncateStr(lastError, 255),
	).Result())
}

func (c *conversationCache) DeadLetterFileProcessingMessage(
	ctx context.Context,
	consumerName string,
	message repository.FileProcessingMessage,
	lastError string,
) (bool, error) {
	if c.client == nil {
		return true, nil
	}
	return fileProcessingScriptResult(deadLetterFileProcessingMessageScript.Run(
		ctx,
		c.client,
		[]string{fileProcessingStreamName, fileProcessingDLQName},
		fileProcessingGroupName,
		consumerName,
		message.ID,
		message.UserID,
		message.FileID,
		message.Retry,
		truncateStr(lastError, 255),
		fileProcessingDLQMaxLen,
	).Result())
}

func fileProcessingScriptResult(result interface{}, err error) (bool, error) {
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	value, ok := result.(int64)
	return ok && value == 1, nil
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

// RegisterGenerationStream records the run owner and conversation without mixing
// ephemeral execution state into persisted conversation records.
func (c *conversationCache) RegisterGenerationStream(ctx context.Context, runID string, userID uint, conversationPublicID string, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	pipe := c.client.Pipeline()
	pipe.Set(ctx, generationStreamOwnerKey(runID), strconv.FormatUint(uint64(userID), 10), ttl)
	pipe.Set(ctx, generationStreamConversationKey(runID), strings.TrimSpace(conversationPublicID), ttl)
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
	value, err := strconv.ParseUint(strings.TrimSpace(raw), 10, strconv.IntSize)
	if err != nil || value == 0 {
		return 0, false, nil
	}
	return uint(value), true, nil
}

// TouchGenerationStreamActive 刷新 run 的活跃租约。
func (c *conversationCache) TouchGenerationStreamActive(ctx context.Context, runID string, userID uint, ttl time.Duration) error {
	if c.client == nil || ttl <= 0 {
		return nil
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	if userID == 0 {
		return nil
	}
	owner := strconv.FormatUint(uint64(userID), 10)
	return touchGenerationStreamActiveScript.Run(ctx, c.client, []string{
		generationStreamOwnerKey(runID),
		generationStreamActiveKey(runID),
		generationStreamActiveIndexKey(userID),
	}, owner, ttl.Milliseconds(), time.Now().Add(ttl).UnixMilli(), runID, generationStreamIndexTTL.Milliseconds()).Err()
}

// ClearGenerationStreamActive 清理 run 的活跃租约。
func (c *conversationCache) ClearGenerationStreamActive(ctx context.Context, runID string, userID uint) error {
	if c.client == nil {
		return nil
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	if userID == 0 {
		return nil
	}
	owner := strconv.FormatUint(uint64(userID), 10)
	return clearGenerationStreamActiveScript.Run(ctx, c.client, []string{
		generationStreamOwnerKey(runID),
		generationStreamActiveKey(runID),
		generationStreamActiveIndexKey(userID),
	}, owner, runID).Err()
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

// ListActiveGenerationStreams returns the user's active runs from a compact
// Redis index. Expired leases and entries reassigned to another user are
// removed opportunistically.
func (c *conversationCache) ListActiveGenerationStreams(ctx context.Context, userID uint) ([]repository.ActiveGenerationStream, error) {
	if c.client == nil || userID == 0 {
		return []repository.ActiveGenerationStream{}, nil
	}
	indexKey := generationStreamActiveIndexKey(userID)
	now := time.Now().UnixMilli()
	pipe := c.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, indexKey, "-inf", strconv.FormatInt(now, 10))
	activeCmd := pipe.ZRangeByScore(ctx, indexKey, &redis.ZRangeBy{
		Min: strconv.FormatInt(now+1, 10),
		Max: "+inf",
	})
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, err
	}
	runIDs := activeCmd.Val()
	if len(runIDs) == 0 {
		return []repository.ActiveGenerationStream{}, nil
	}
	keys := make([]string, 0, len(runIDs)*2)
	for _, runID := range runIDs {
		keys = append(keys, generationStreamOwnerKey(runID), generationStreamConversationKey(runID))
	}
	metadata, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	items := make([]repository.ActiveGenerationStream, 0, len(runIDs))
	staleRunIDs := make([]interface{}, 0)
	wantedOwner := strconv.FormatUint(uint64(userID), 10)
	for index, runID := range runIDs {
		owner, _ := metadata[index*2].(string)
		conversationPublicID, _ := metadata[index*2+1].(string)
		conversationPublicID = strings.TrimSpace(conversationPublicID)
		if strings.TrimSpace(owner) != wantedOwner || conversationPublicID == "" {
			staleRunIDs = append(staleRunIDs, runID)
			continue
		}
		items = append(items, repository.ActiveGenerationStream{
			RunID:                runID,
			ConversationPublicID: conversationPublicID,
		})
	}
	if len(staleRunIDs) > 0 {
		_ = c.client.ZRem(ctx, indexKey, staleRunIDs...).Err()
	}
	return items, nil
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

// AppendGenerationStreamEvent 原子追加生成事件，并同步维护可见文本快照。
func (c *conversationCache) AppendGenerationStreamEvent(ctx context.Context, runID string, input repository.GenerationStreamAppend, maxEvents int64, ttl time.Duration) (repository.GenerationStreamMessage, error) {
	if c.client == nil {
		return repository.GenerationStreamMessage{}, nil
	}
	if maxEvents <= 0 {
		maxEvents = 1024
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	result, err := appendGenerationStreamEventScript.Run(
		ctx,
		c.client,
		[]string{
			generationStreamSeqKey(runID),
			generationStreamEventsKey(runID),
			generationStreamTextKey(runID),
			generationStreamTextSeqKey(runID),
		},
		input.PayloadJSON,
		maxEvents,
		ttl.Milliseconds(),
		input.TextDelta,
	).Result()
	if err != nil {
		return repository.GenerationStreamMessage{}, err
	}
	values, ok := result.([]interface{})
	if !ok || len(values) != 2 {
		return repository.GenerationStreamMessage{}, errors.New("invalid generation stream append result")
	}
	id := strings.TrimSpace(getStringVal(values[0]))
	seq := getInt64Val(values[1])
	if id == "" || seq <= 0 {
		return repository.GenerationStreamMessage{}, errors.New("invalid generation stream append metadata")
	}
	return repository.GenerationStreamMessage{ID: id, Seq: seq, PayloadJSON: input.PayloadJSON}, nil
}

// GetGenerationStreamTextSnapshot 原子读取完整可见文本及其最后事件序号。
func (c *conversationCache) GetGenerationStreamTextSnapshot(ctx context.Context, runID string) (repository.GenerationStreamTextSnapshot, bool, error) {
	if c.client == nil {
		return repository.GenerationStreamTextSnapshot{}, false, nil
	}
	values, err := c.client.MGet(
		ctx,
		generationStreamTextKey(runID),
		generationStreamTextSeqKey(runID),
	).Result()
	if err != nil {
		return repository.GenerationStreamTextSnapshot{}, false, err
	}
	if len(values) != 2 || values[0] == nil || values[1] == nil {
		return repository.GenerationStreamTextSnapshot{}, false, nil
	}
	seq := getInt64Val(values[1])
	if seq <= 0 {
		return repository.GenerationStreamTextSnapshot{}, false, nil
	}
	return repository.GenerationStreamTextSnapshot{
		Seq:     seq,
		Content: getStringVal(values[0]),
	}, true, nil
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

// ResetGenerationStreamEvents 清空恢复流事件，阻止撤回内容在重连时被回放。
func (c *conversationCache) ResetGenerationStreamEvents(ctx context.Context, runID string) error {
	if c.client == nil {
		return nil
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil
	}
	// Keep seq key so subsequent appends stay monotonic for reconnect cursors.
	return c.client.Del(
		ctx,
		generationStreamEventsKey(runID),
		generationStreamTextKey(runID),
		generationStreamTextSeqKey(runID),
	).Err()
}

// ExpireGenerationStream 设置生成流相关键的过期时间。
func (c *conversationCache) ExpireGenerationStream(ctx context.Context, runID string, ttl time.Duration) error {
	if c.client == nil || ttl <= 0 {
		return nil
	}
	pipe := c.client.Pipeline()
	pipe.Expire(ctx, generationStreamEventsKey(runID), ttl)
	pipe.Expire(ctx, generationStreamSeqKey(runID), ttl)
	pipe.Expire(ctx, generationStreamTextKey(runID), ttl)
	pipe.Expire(ctx, generationStreamTextSeqKey(runID), ttl)
	pipe.Expire(ctx, generationStreamOwnerKey(runID), ttl)
	pipe.Expire(ctx, generationStreamConversationKey(runID), ttl)
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

func generationStreamTextKey(runID string) string {
	return generationStreamKeyPrefix + strings.TrimSpace(runID) + ":text"
}

func generationStreamTextSeqKey(runID string) string {
	return generationStreamKeyPrefix + strings.TrimSpace(runID) + ":text_seq"
}

func generationStreamOwnerKey(runID string) string {
	return generationStreamKeyPrefix + strings.TrimSpace(runID) + ":owner"
}

func generationStreamConversationKey(runID string) string {
	return generationStreamKeyPrefix + strings.TrimSpace(runID) + ":conversation"
}

func generationStreamActiveIndexKey(userID uint) string {
	return generationStreamKeyPrefix + "user:" + strconv.FormatUint(uint64(userID), 10) + ":active"
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
