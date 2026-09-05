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
	fileEmbeddingStreamName  = "file_embedding_v1"
	fileEmbeddingDLQName     = "file_embedding_v1_dlq"
	fileEmbeddingGroupName   = "file_embedding_workers"
	fileProcessingMinIdle    = 45 * time.Second
	fileProcessingDLQMaxLen  = 10_000

	generationStreamKeyPrefix = "conversation:generation:"
	generationStreamIndexTTL  = 2 * time.Hour
	activeGenerationEventID   = "active_events_v1"
)

type fileQueueConfig struct {
	stream string
	dlq    string
	group  string
	queue  repository.FileProcessingQueue
}

var claimGenerationStreamScript = redis.NewScript(`
local current_execution = redis.call("GET", KEYS[1])
if current_execution then
	if current_execution == ARGV[1]
		and redis.call("GET", KEYS[2]) == ARGV[2]
		and redis.call("GET", KEYS[3]) == ARGV[3] then
		redis.call("PEXPIRE", KEYS[1], ARGV[4])
		redis.call("PEXPIRE", KEYS[2], ARGV[5])
		redis.call("PEXPIRE", KEYS[3], ARGV[5])
		redis.call("ZADD", KEYS[5], ARGV[6], ARGV[7])
		redis.call("PEXPIRE", KEYS[5], ARGV[8])
		return 1
	end
	return 0
end
local current_owner = redis.call("GET", KEYS[2])
if current_owner then
	return 0
end
local current_conversation = redis.call("GET", KEYS[3])
if current_conversation then
	return 0
end
redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[4])
redis.call("SET", KEYS[2], ARGV[2], "PX", ARGV[5])
redis.call("SET", KEYS[3], ARGV[3], "PX", ARGV[5])
redis.call("DEL", KEYS[4], KEYS[6], KEYS[7], KEYS[8], KEYS[9], KEYS[10])
redis.call("ZADD", KEYS[5], ARGV[6], ARGV[7])
redis.call("PEXPIRE", KEYS[5], ARGV[8])
return 1
`)

// appendGenerationStreamEventScript 原子维护执行权隔离、事件序号、有界回放和恢复快照。
var appendGenerationStreamEventScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) ~= ARGV[10] then
	return {"0"}
end
local events_missing = redis.call("EXISTS", KEYS[3]) == 0
local has_text_delta = ARGV[4] ~= ""
local text_missing = false
if has_text_delta then
	text_missing = redis.call("EXISTS", KEYS[4]) == 0
end
local has_think_update = ARGV[5] == "1"
local think_content_missing = false
local think_meta_missing = false
if has_think_update then
	think_content_missing = redis.call("EXISTS", KEYS[6]) == 0
	think_meta_missing = redis.call("EXISTS", KEYS[7]) == 0
end
local seq = redis.call("INCR", KEYS[2])
if has_text_delta then
	redis.call("APPEND", KEYS[4], ARGV[4])
	redis.call("SET", KEYS[5], tostring(seq), "KEEPTTL")
end
if has_think_update then
	local previous_round = redis.call("HGET", KEYS[7], "round") or ""
	local next_round = ARGV[8]
	if next_round == "" then
		next_round = previous_round
	end
	if next_round ~= "" and previous_round ~= "" and next_round ~= previous_round then
		redis.call("DEL", KEYS[6])
		think_content_missing = true
	end
	if ARGV[6] == "1" then
		redis.call("SET", KEYS[6], ARGV[7], "KEEPTTL")
	else
		redis.call("APPEND", KEYS[6], ARGV[7])
	end
	redis.call("HSET", KEYS[7],
		"seq", tostring(seq),
		"round", next_round,
		"metadata", ARGV[9]
	)
end
local id = redis.call(
	"XADD",
	KEYS[3],
	"MAXLEN", "~", ARGV[2],
	"*",
	"seq", tostring(seq),
	"payload", ARGV[1]
)

	redis.call("PEXPIRE", KEYS[2], ARGV[3])
if events_missing then
	redis.call("PEXPIRE", KEYS[3], ARGV[3])
end
if has_text_delta and text_missing then
	redis.call("PEXPIRE", KEYS[4], ARGV[3])
	redis.call("PEXPIRE", KEYS[5], ARGV[3])
end
if has_think_update and think_content_missing then
	redis.call("PEXPIRE", KEYS[6], ARGV[3])
end
if has_think_update and think_meta_missing then
	redis.call("PEXPIRE", KEYS[7], ARGV[3])
end

return {"1", id, tostring(seq)}
`)

var appendActiveGenerationEventScript = redis.NewScript(`
local seq = redis.call("INCR", KEYS[1])
local id = redis.call(
	"XADD",
	KEYS[2],
	"MAXLEN", "~", ARGV[2],
	"*",
	"seq", tostring(seq),
	"payload", ARGV[1]
)
redis.call("PEXPIRE", KEYS[1], ARGV[3])
redis.call("PEXPIRE", KEYS[2], ARGV[3])
return {id, tostring(seq)}
`)

var getGenerationStreamUpstreamThinkSnapshotScript = redis.NewScript(`
if redis.call("EXISTS", KEYS[1]) == 0 or redis.call("EXISTS", KEYS[2]) == 0 then
	return {"0"}
end
return {
	"1",
	redis.call("GET", KEYS[1]) or "",
	redis.call("HGET", KEYS[2], "seq") or "",
	redis.call("HGET", KEYS[2], "round") or "",
	redis.call("HGET", KEYS[2], "metadata") or ""
}
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
	"last_error", ARGV[7],
	"kind", ARGV[8],
	"embedding_signature", ARGV[9],
	"embedding_host", ARGV[10]
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
	"XADD", KEYS[2], "MAXLEN", ARGV[11], "*",
	"user_id", ARGV[4],
	"file_id", ARGV[5],
	"retry", ARGV[6],
	"last_error", ARGV[7],
	"kind", ARGV[8],
	"embedding_signature", ARGV[9],
	"embedding_host", ARGV[10]
)
redis.call("XACK", KEYS[1], ARGV[1], ARGV[3])
redis.call("XDEL", KEYS[1], ARGV[3])
return 1
`)

var renewGenerationStreamLeaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) ~= ARGV[1]
	or redis.call("GET", KEYS[2]) ~= ARGV[2]
	or redis.call("GET", KEYS[3]) ~= ARGV[3] then
	return 0
end
redis.call("PEXPIRE", KEYS[1], ARGV[4])
redis.call("PEXPIRE", KEYS[2], ARGV[5])
redis.call("PEXPIRE", KEYS[3], ARGV[5])
redis.call("ZADD", KEYS[4], ARGV[6], ARGV[7])
redis.call("PEXPIRE", KEYS[4], ARGV[8])
return 1
`)

var requestGenerationStreamCancelScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) ~= ARGV[1] or redis.call("EXISTS", KEYS[2]) == 0 then
	return 0
end
redis.call("SET", KEYS[3], "1", "PX", ARGV[2])
return 1
`)

var completeGenerationStreamScript = redis.NewScript(`
if redis.call("GET", KEYS[2]) ~= ARGV[2] or redis.call("GET", KEYS[4]) ~= ARGV[4] then
	return 0
end
local current_execution = redis.call("GET", KEYS[1])
if current_execution and current_execution ~= ARGV[1] then
	return 0
end
redis.call("DEL", KEYS[1])
redis.call("ZREM", KEYS[3], ARGV[3])
	for index = 2, 11 do
	if index ~= 3 then
		redis.call("PEXPIRE", KEYS[index], ARGV[5])
	end
end
return 1
`)

var abandonGenerationStreamScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) ~= ARGV[1] or redis.call("GET", KEYS[2]) ~= ARGV[2] then
	return 0
end
redis.call("ZREM", KEYS[3], ARGV[3])
redis.call("DEL", KEYS[1], KEYS[2], KEYS[4], KEYS[5], KEYS[6], KEYS[7], KEYS[8], KEYS[9], KEYS[10], KEYS[11])
return 1
`)

var resetGenerationStreamEventsScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) ~= ARGV[1] then
	return 0
end
redis.call("DEL", KEYS[2], KEYS[3], KEYS[4], KEYS[5], KEYS[6])
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
	for _, queue := range []fileQueueConfig{processingQueueConfig(), embeddingQueueConfig()} {
		err := c.client.XGroupCreateMkStream(ctx, queue.stream, queue.group, "0").Err()
		if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
			return err
		}
	}
	return nil
}

// EnqueueFileProcessing 将文件处理任务推入 Stream 队列。
func (c *conversationCache) EnqueueFileProcessing(ctx context.Context, userID uint, fileID string, retry int, lastError string) error {
	if c.client == nil {
		return nil
	}
	values := map[string]any{
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

// EnqueueFileEmbedding 将显式向量化任务推入独立的可恢复 Stream。
func (c *conversationCache) EnqueueFileEmbedding(
	ctx context.Context,
	userID uint,
	fileID string,
	embeddingSignature string,
	embeddingHost string,
) error {
	fileID = strings.TrimSpace(fileID)
	embeddingSignature = strings.TrimSpace(embeddingSignature)
	embeddingHost = strings.TrimRight(strings.TrimSpace(embeddingHost), "/")
	if fileID == "" || embeddingSignature == "" || embeddingHost == "" {
		return repository.ErrInvalidInput
	}
	if c.client == nil {
		return nil
	}
	_, err := c.client.XAdd(ctx, &redis.XAddArgs{
		Stream: fileEmbeddingStreamName,
		Values: map[string]any{
			"user_id":             userID,
			"file_id":             fileID,
			"retry":               0,
			"kind":                repository.FileProcessingKindEmbedding,
			"embedding_signature": embeddingSignature,
			"embedding_host":      embeddingHost,
		},
	}).Result()
	return err
}

// ClaimTimedOutFileProcessingMessages 认领超时未确认的 pending 任务，避免 worker 重启后任务永久卡住。
func (c *conversationCache) ClaimTimedOutFileProcessingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
	return c.claimTimedOutFileMessages(ctx, consumerName, processingQueueConfig())
}

func (c *conversationCache) ClaimTimedOutFileEmbeddingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
	return c.claimTimedOutFileMessages(ctx, consumerName, embeddingQueueConfig())
}

func (c *conversationCache) claimTimedOutFileMessages(
	ctx context.Context,
	consumerName string,
	queue fileQueueConfig,
) ([]repository.FileProcessingMessage, error) {
	if c.client == nil {
		return nil, nil
	}
	pending, err := c.client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: queue.stream,
		Group:  queue.group,
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
		Stream:   queue.stream,
		Group:    queue.group,
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
	return c.decodeFileProcessingMessages(ctx, consumerName, claimed, true, queue)
}

// ReadFileProcessingMessages 阻塞读取未处理消息（最多 1 条，5s 超时）。
func (c *conversationCache) ReadFileProcessingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
	return c.readFileMessages(ctx, consumerName, processingQueueConfig())
}

func (c *conversationCache) ReadFileEmbeddingMessages(ctx context.Context, consumerName string) ([]repository.FileProcessingMessage, error) {
	return c.readFileMessages(ctx, consumerName, embeddingQueueConfig())
}

func (c *conversationCache) readFileMessages(
	ctx context.Context,
	consumerName string,
	queue fileQueueConfig,
) ([]repository.FileProcessingMessage, error) {
	if c.client == nil {
		return nil, nil
	}
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    queue.group,
		Consumer: consumerName,
		Streams:  []string{queue.stream, ">"},
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
		parsed, parseErr := c.decodeFileProcessingMessages(ctx, consumerName, stream.Messages, false, queue)
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
	queue fileQueueConfig,
) ([]repository.FileProcessingMessage, error) {
	parsedMessages := make([]repository.FileProcessingMessage, 0, len(messages))
	for _, msg := range messages {
		parsed, err := parseFileProcessingMessage(msg)
		if err != nil {
			quarantined, quarantineErr := c.deadLetterInvalidFileProcessingMessage(ctx, consumerName, msg, err, queue)
			if quarantineErr != nil {
				return nil, fmt.Errorf("dead-letter invalid file processing message %q: %w", msg.ID, quarantineErr)
			}
			if !quarantined {
				return nil, fmt.Errorf("dead-letter invalid file processing message %q: message ownership lost", msg.ID)
			}
			continue
		}
		parsed.Reclaimed = reclaimed
		parsed.Queue = queue.queue
		parsedMessages = append(parsedMessages, parsed)
	}
	return parsedMessages, nil
}

func parseFileProcessingMessage(msg redis.XMessage) (repository.FileProcessingMessage, error) {
	kind := strings.TrimSpace(getOptionalStringVal(msg.Values, "kind"))
	if kind != "" && kind != repository.FileProcessingKindEmbedding {
		return repository.FileProcessingMessage{}, fmt.Errorf("invalid processing kind %q", kind)
	}
	userID, err := strconv.ParseUint(strings.TrimSpace(getStringVal(msg.Values["user_id"])), 10, strconv.IntSize)
	if err != nil || (userID == 0 && kind != repository.FileProcessingKindEmbedding) {
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
		lastError = truncateStr(getStringVal(rawLastError), 255)
	}
	embeddingSignature := strings.TrimSpace(getOptionalStringVal(msg.Values, "embedding_signature"))
	embeddingHost := strings.TrimRight(strings.TrimSpace(getOptionalStringVal(msg.Values, "embedding_host")), "/")
	if kind == repository.FileProcessingKindEmbedding && (embeddingSignature == "" || embeddingHost == "") {
		return repository.FileProcessingMessage{}, errors.New("invalid embedding queue metadata")
	}

	return repository.FileProcessingMessage{
		ID:                 msg.ID,
		UserID:             uint(userID),
		FileID:             strings.TrimSpace(getStringVal(msg.Values["file_id"])),
		Retry:              retry,
		LastError:          lastError,
		Kind:               kind,
		EmbeddingSignature: embeddingSignature,
		EmbeddingHost:      embeddingHost,
	}, nil
}

func (c *conversationCache) deadLetterInvalidFileProcessingMessage(
	ctx context.Context,
	consumerName string,
	message redis.XMessage,
	parseErr error,
	queue fileQueueConfig,
) (bool, error) {
	lastError := "invalid queue message"

	return fileProcessingScriptResult(deadLetterFileProcessingMessageScript.Run(
		ctx,
		c.client,
		[]string{queue.stream, queue.dlq},
		queue.group,
		consumerName,
		message.ID,
		getStringVal(message.Values["user_id"]),
		getStringVal(message.Values["file_id"]),
		getStringVal(message.Values["retry"]),
		truncateStr(lastError, 255),
		getOptionalStringVal(message.Values, "kind"),
		getOptionalStringVal(message.Values, "embedding_signature"),
		getOptionalStringVal(message.Values, "embedding_host"),
		fileProcessingDLQMaxLen,
	).Result())
}

// RenewFileProcessingMessageLease 刷新执行中消息的空闲时间，避免长任务被其他 worker 重复认领。
func (c *conversationCache) RenewFileProcessingMessageLease(ctx context.Context, consumerName string, message repository.FileProcessingMessage) (bool, error) {
	if c.client == nil || strings.TrimSpace(consumerName) == "" || strings.TrimSpace(message.ID) == "" {
		return true, nil
	}
	queue := redisQueueForMessage(message)
	return fileProcessingScriptResult(renewFileProcessingLeaseScript.Run(
		ctx,
		c.client,
		[]string{queue.stream},
		queue.group,
		consumerName,
		message.ID,
	).Result())
}

func (c *conversationCache) SettleFileProcessingMessage(ctx context.Context, consumerName string, message repository.FileProcessingMessage) (bool, error) {
	if c.client == nil {
		return true, nil
	}
	queue := redisQueueForMessage(message)
	return fileProcessingScriptResult(settleFileProcessingMessageScript.Run(
		ctx,
		c.client,
		[]string{queue.stream},
		queue.group,
		consumerName,
		message.ID,
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
	queue := redisQueueForMessage(message)
	return fileProcessingScriptResult(requeueFileProcessingMessageScript.Run(
		ctx,
		c.client,
		[]string{queue.stream},
		queue.group,
		consumerName,
		message.ID,
		message.UserID,
		message.FileID,
		retry,
		truncateStr(lastError, 255),
		message.Kind,
		message.EmbeddingSignature,
		message.EmbeddingHost,
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
	queue := redisQueueForMessage(message)
	return fileProcessingScriptResult(deadLetterFileProcessingMessageScript.Run(
		ctx,
		c.client,
		[]string{queue.stream, queue.dlq},
		queue.group,
		consumerName,
		message.ID,
		message.UserID,
		message.FileID,
		message.Retry,
		truncateStr(lastError, 255),
		message.Kind,
		message.EmbeddingSignature,
		message.EmbeddingHost,
		fileProcessingDLQMaxLen,
	).Result())
}

func processingQueueConfig() fileQueueConfig {
	return fileQueueConfig{
		stream: fileProcessingStreamName,
		dlq:    fileProcessingDLQName,
		group:  fileProcessingGroupName,
		queue:  repository.FileProcessingQueueDefault,
	}
}

func embeddingQueueConfig() fileQueueConfig {
	return fileQueueConfig{
		stream: fileEmbeddingStreamName,
		dlq:    fileEmbeddingDLQName,
		group:  fileEmbeddingGroupName,
		queue:  repository.FileProcessingQueueEmbedding,
	}
}

func redisQueueForMessage(message repository.FileProcessingMessage) fileQueueConfig {
	if message.Queue == repository.FileProcessingQueueEmbedding ||
		(message.Queue == "" && message.Kind == repository.FileProcessingKindEmbedding) {
		return embeddingQueueConfig()
	}
	return processingQueueConfig()
}

func fileProcessingScriptResult(result any, err error) (bool, error) {
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

// ClaimGenerationStream 原子声明一次运行的唯一写入执行权。
func (c *conversationCache) ClaimGenerationStream(
	ctx context.Context,
	lease repository.GenerationStreamLease,
	leaseTTL time.Duration,
	ownershipTTL time.Duration,
) (bool, error) {
	if c.client == nil {
		return true, nil
	}
	lease.RunID = strings.TrimSpace(lease.RunID)
	lease.ExecutionID = strings.TrimSpace(lease.ExecutionID)
	lease.ConversationPublicID = strings.TrimSpace(lease.ConversationPublicID)
	if lease.RunID == "" || lease.ExecutionID == "" || lease.UserID == 0 || lease.ConversationPublicID == "" {
		return false, nil
	}
	if leaseTTL <= 0 {
		leaseTTL = time.Minute
	}
	if ownershipTTL < leaseTTL {
		ownershipTTL = leaseTTL
	}
	claimed, err := claimGenerationStreamScript.Run(ctx, c.client, []string{
		generationStreamActiveKey(lease.RunID),
		generationStreamOwnerKey(lease.RunID),
		generationStreamConversationKey(lease.RunID),
		generationStreamCancelKey(lease.RunID),
		generationStreamActiveIndexKey(lease.UserID),
		generationStreamEventsKey(lease.RunID),
		generationStreamTextKey(lease.RunID),
		generationStreamTextSeqKey(lease.RunID),
		generationStreamUpstreamThinkContentKey(lease.RunID),
		generationStreamUpstreamThinkMetaKey(lease.RunID),
	},
		lease.ExecutionID,
		strconv.FormatUint(uint64(lease.UserID), 10),
		lease.ConversationPublicID,
		leaseTTL.Milliseconds(),
		ownershipTTL.Milliseconds(),
		time.Now().Add(leaseTTL).UnixMilli(),
		lease.RunID,
		generationStreamIndexTTL.Milliseconds(),
	).Int()
	if err != nil {
		return false, err
	}
	return claimed == 1, nil
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

// RenewGenerationStreamLease 仅为当前执行续租。
func (c *conversationCache) RenewGenerationStreamLease(
	ctx context.Context,
	lease repository.GenerationStreamLease,
	leaseTTL time.Duration,
	ownershipTTL time.Duration,
) (bool, error) {
	if c.client == nil {
		return true, nil
	}
	lease.RunID = strings.TrimSpace(lease.RunID)
	lease.ExecutionID = strings.TrimSpace(lease.ExecutionID)
	lease.ConversationPublicID = strings.TrimSpace(lease.ConversationPublicID)
	if lease.RunID == "" || lease.ExecutionID == "" || lease.UserID == 0 || lease.ConversationPublicID == "" || leaseTTL <= 0 {
		return false, nil
	}
	if ownershipTTL < leaseTTL {
		ownershipTTL = leaseTTL
	}
	owner := strconv.FormatUint(uint64(lease.UserID), 10)
	renewed, err := renewGenerationStreamLeaseScript.Run(ctx, c.client, []string{
		generationStreamActiveKey(lease.RunID),
		generationStreamOwnerKey(lease.RunID),
		generationStreamConversationKey(lease.RunID),
		generationStreamActiveIndexKey(lease.UserID),
	},
		lease.ExecutionID,
		owner,
		lease.ConversationPublicID,
		leaseTTL.Milliseconds(),
		ownershipTTL.Milliseconds(),
		time.Now().Add(leaseTTL).UnixMilli(),
		lease.RunID,
		generationStreamIndexTTL.Milliseconds(),
	).Int()
	if err != nil {
		return false, err
	}
	return renewed == 1, nil
}

// CompleteGenerationStream 释放执行租约并保留可恢复数据。
func (c *conversationCache) CompleteGenerationStream(ctx context.Context, lease repository.GenerationStreamLease, retention time.Duration) (bool, error) {
	if c.client == nil {
		return true, nil
	}
	lease.RunID = strings.TrimSpace(lease.RunID)
	lease.ExecutionID = strings.TrimSpace(lease.ExecutionID)
	if lease.RunID == "" || lease.ExecutionID == "" || lease.UserID == 0 || retention <= 0 {
		return false, nil
	}
	owner := strconv.FormatUint(uint64(lease.UserID), 10)
	completed, err := completeGenerationStreamScript.Run(ctx, c.client, []string{
		generationStreamActiveKey(lease.RunID),
		generationStreamOwnerKey(lease.RunID),
		generationStreamActiveIndexKey(lease.UserID),
		generationStreamConversationKey(lease.RunID),
		generationStreamCancelKey(lease.RunID),
		generationStreamEventsKey(lease.RunID),
		generationStreamSeqKey(lease.RunID),
		generationStreamTextKey(lease.RunID),
		generationStreamTextSeqKey(lease.RunID),
		generationStreamUpstreamThinkContentKey(lease.RunID),
		generationStreamUpstreamThinkMetaKey(lease.RunID),
	}, lease.ExecutionID, owner, lease.RunID, lease.ConversationPublicID, retention.Milliseconds()).Int()
	if err != nil {
		return false, err
	}
	return completed == 1, nil
}

// AbandonGenerationStream 移除未能在本节点完成注册的执行权声明。
func (c *conversationCache) AbandonGenerationStream(ctx context.Context, lease repository.GenerationStreamLease) (bool, error) {
	if c.client == nil {
		return true, nil
	}
	lease.RunID = strings.TrimSpace(lease.RunID)
	lease.ExecutionID = strings.TrimSpace(lease.ExecutionID)
	if lease.RunID == "" || lease.ExecutionID == "" || lease.UserID == 0 {
		return false, nil
	}
	owner := strconv.FormatUint(uint64(lease.UserID), 10)
	abandoned, err := abandonGenerationStreamScript.Run(ctx, c.client, []string{
		generationStreamActiveKey(lease.RunID),
		generationStreamOwnerKey(lease.RunID),
		generationStreamActiveIndexKey(lease.UserID),
		generationStreamConversationKey(lease.RunID),
		generationStreamCancelKey(lease.RunID),
		generationStreamEventsKey(lease.RunID),
		generationStreamSeqKey(lease.RunID),
		generationStreamTextKey(lease.RunID),
		generationStreamTextSeqKey(lease.RunID),
		generationStreamUpstreamThinkContentKey(lease.RunID),
		generationStreamUpstreamThinkMetaKey(lease.RunID),
	}, lease.ExecutionID, owner, lease.RunID).Int()
	if err != nil {
		return false, err
	}
	return abandoned == 1, nil
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
	keys := make([]string, 0, len(runIDs)*3)
	for _, runID := range runIDs {
		keys = append(
			keys,
			generationStreamActiveKey(runID),
			generationStreamOwnerKey(runID),
			generationStreamConversationKey(runID),
		)
	}
	metadata, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	items := make([]repository.ActiveGenerationStream, 0, len(runIDs))
	staleRunIDs := make([]any, 0)
	wantedOwner := strconv.FormatUint(uint64(userID), 10)
	for index, runID := range runIDs {
		activeExecution, _ := metadata[index*3].(string)
		owner, _ := metadata[index*3+1].(string)
		conversationPublicID, _ := metadata[index*3+2].(string)
		conversationPublicID = strings.TrimSpace(conversationPublicID)
		if strings.TrimSpace(activeExecution) == "" || strings.TrimSpace(owner) != wantedOwner || conversationPublicID == "" {
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

// RequestGenerationStreamCancel 将用户所属的活动运行标记为已取消。
func (c *conversationCache) RequestGenerationStreamCancel(ctx context.Context, runID string, userID uint, ttl time.Duration) (bool, error) {
	if c.client == nil {
		return true, nil
	}
	runID = strings.TrimSpace(runID)
	if runID == "" || userID == 0 || ttl <= 0 {
		return false, nil
	}
	requested, err := requestGenerationStreamCancelScript.Run(ctx, c.client, []string{
		generationStreamOwnerKey(runID),
		generationStreamActiveKey(runID),
		generationStreamCancelKey(runID),
	}, strconv.FormatUint(uint64(userID), 10), ttl.Milliseconds()).Int()
	if err != nil {
		return false, err
	}
	return requested == 1, nil
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

// AppendGenerationStreamEvent 为当前执行原子追加事件。
func (c *conversationCache) AppendGenerationStreamEvent(
	ctx context.Context,
	lease repository.GenerationStreamLease,
	input repository.GenerationStreamAppend,
	maxEvents int64,
	ttl time.Duration,
) (repository.GenerationStreamMessage, bool, error) {
	if c.client == nil {
		return repository.GenerationStreamMessage{}, true, nil
	}
	lease.RunID = strings.TrimSpace(lease.RunID)
	lease.ExecutionID = strings.TrimSpace(lease.ExecutionID)
	if lease.RunID == "" || lease.ExecutionID == "" {
		return repository.GenerationStreamMessage{}, false, nil
	}
	if maxEvents <= 0 {
		maxEvents = 1024
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	hasUpstreamThink := "0"
	upstreamThinkReplace := "0"
	upstreamThinkContent := ""
	upstreamThinkRoundID := ""
	upstreamThinkMetadata := ""
	if input.UpstreamThink != nil {
		hasUpstreamThink = "1"
		if input.UpstreamThink.Replace {
			upstreamThinkReplace = "1"
			upstreamThinkContent = input.UpstreamThink.ContentMarkdown
		} else {
			upstreamThinkContent = input.UpstreamThink.Delta
		}
		upstreamThinkRoundID = input.UpstreamThink.RoundID
		upstreamThinkMetadata = input.UpstreamThink.MetadataJSON
	}
	result, err := appendGenerationStreamEventScript.Run(
		ctx,
		c.client,
		[]string{
			generationStreamActiveKey(lease.RunID),
			generationStreamSeqKey(lease.RunID),
			generationStreamEventsKey(lease.RunID),
			generationStreamTextKey(lease.RunID),
			generationStreamTextSeqKey(lease.RunID),
			generationStreamUpstreamThinkContentKey(lease.RunID),
			generationStreamUpstreamThinkMetaKey(lease.RunID),
		},
		input.PayloadJSON,
		maxEvents,
		ttl.Milliseconds(),
		input.TextDelta,
		hasUpstreamThink,
		upstreamThinkReplace,
		upstreamThinkContent,
		upstreamThinkRoundID,
		upstreamThinkMetadata,
		lease.ExecutionID,
	).Result()
	if err != nil {
		return repository.GenerationStreamMessage{}, false, err
	}
	values, ok := result.([]any)
	if !ok || len(values) == 0 {
		return repository.GenerationStreamMessage{}, false, errors.New("invalid generation stream append result")
	}
	if getStringVal(values[0]) != "1" {
		return repository.GenerationStreamMessage{}, false, nil
	}
	if len(values) != 3 {
		return repository.GenerationStreamMessage{}, false, errors.New("invalid generation stream append result")
	}
	id := strings.TrimSpace(getStringVal(values[1]))
	seq := getInt64Val(values[2])
	if id == "" || seq <= 0 {
		return repository.GenerationStreamMessage{}, false, errors.New("invalid generation stream append metadata")
	}
	return repository.GenerationStreamMessage{ID: id, Seq: seq, PayloadJSON: input.PayloadJSON}, true, nil
}

// AppendActiveGenerationEvent 追加一条跨进程共享的活动状态事件。
func (c *conversationCache) AppendActiveGenerationEvent(
	ctx context.Context,
	input repository.GenerationStreamAppend,
	maxEvents int64,
	ttl time.Duration,
) (repository.GenerationStreamMessage, error) {
	if c.client == nil {
		return repository.GenerationStreamMessage{}, nil
	}
	if maxEvents <= 0 {
		maxEvents = 1024
	}
	if ttl <= 0 {
		ttl = time.Minute
	}
	result, err := appendActiveGenerationEventScript.Run(
		ctx,
		c.client,
		[]string{
			generationStreamSeqKey(activeGenerationEventID),
			generationStreamEventsKey(activeGenerationEventID),
		},
		input.PayloadJSON,
		maxEvents,
		ttl.Milliseconds(),
	).Result()
	if err != nil {
		return repository.GenerationStreamMessage{}, err
	}
	values, ok := result.([]any)
	if !ok || len(values) != 2 {
		return repository.GenerationStreamMessage{}, errors.New("invalid active generation event append result")
	}
	id := strings.TrimSpace(getStringVal(values[0]))
	seq := getInt64Val(values[1])
	if id == "" || seq <= 0 {
		return repository.GenerationStreamMessage{}, errors.New("invalid active generation event append metadata")
	}
	return repository.GenerationStreamMessage{ID: id, Seq: seq, PayloadJSON: input.PayloadJSON}, nil
}

// GetGenerationStreamUpstreamThinkSnapshot 原子读取当前思考轮次的完整恢复快照。
func (c *conversationCache) GetGenerationStreamUpstreamThinkSnapshot(ctx context.Context, runID string) (repository.GenerationStreamUpstreamThinkSnapshot, bool, error) {
	if c.client == nil {
		return repository.GenerationStreamUpstreamThinkSnapshot{}, false, nil
	}
	result, err := getGenerationStreamUpstreamThinkSnapshotScript.Run(
		ctx,
		c.client,
		[]string{
			generationStreamUpstreamThinkContentKey(runID),
			generationStreamUpstreamThinkMetaKey(runID),
		},
	).Result()
	if err != nil {
		return repository.GenerationStreamUpstreamThinkSnapshot{}, false, err
	}
	values, ok := result.([]any)
	if !ok || len(values) == 0 || getStringVal(values[0]) != "1" {
		return repository.GenerationStreamUpstreamThinkSnapshot{}, false, nil
	}
	if len(values) != 5 {
		return repository.GenerationStreamUpstreamThinkSnapshot{}, false, nil
	}
	seq := getInt64Val(values[2])
	if seq <= 0 {
		return repository.GenerationStreamUpstreamThinkSnapshot{}, false, nil
	}
	return repository.GenerationStreamUpstreamThinkSnapshot{
		Seq:             seq,
		RoundID:         getStringVal(values[3]),
		ContentMarkdown: getStringVal(values[1]),
		MetadataJSON:    getStringVal(values[4]),
	}, true, nil
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

// ResetGenerationStreamEvents 仅清空当前执行保留的输出事件。
func (c *conversationCache) ResetGenerationStreamEvents(ctx context.Context, lease repository.GenerationStreamLease) (bool, error) {
	if c.client == nil {
		return true, nil
	}
	lease.RunID = strings.TrimSpace(lease.RunID)
	lease.ExecutionID = strings.TrimSpace(lease.ExecutionID)
	if lease.RunID == "" || lease.ExecutionID == "" {
		return false, nil
	}
	// Keep seq key so subsequent appends stay monotonic for reconnect cursors.
	reset, err := resetGenerationStreamEventsScript.Run(ctx, c.client, []string{
		generationStreamActiveKey(lease.RunID),
		generationStreamEventsKey(lease.RunID),
		generationStreamTextKey(lease.RunID),
		generationStreamTextSeqKey(lease.RunID),
		generationStreamUpstreamThinkContentKey(lease.RunID),
		generationStreamUpstreamThinkMetaKey(lease.RunID),
	}, lease.ExecutionID).Int()
	if err != nil {
		return false, err
	}
	return reset == 1, nil
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

func generationStreamUpstreamThinkContentKey(runID string) string {
	return generationStreamKeyPrefix + strings.TrimSpace(runID) + ":upstream_think"
}

func generationStreamUpstreamThinkMetaKey(runID string) string {
	return generationStreamKeyPrefix + strings.TrimSpace(runID) + ":upstream_think_meta"
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

func getStringVal(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", raw)
	}
}

func getOptionalStringVal(values map[string]any, key string) string {
	raw, ok := values[key]
	if !ok || raw == nil {
		return ""
	}
	return getStringVal(raw)
}

func getInt64Val(raw any) int64 {
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
