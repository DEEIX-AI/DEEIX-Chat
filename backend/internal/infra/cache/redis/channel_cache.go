package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

// circuitProbeTTLSec 半开 probe 令牌的 TTL（秒）。
const circuitProbeTTLSec = 30

// circuitManualOpenDuration 手动打开熔断的持续时间。
const circuitManualOpenDuration = 24 * time.Hour

// circuitBreakerCheckScript 检查熔断状态的 Lua 脚本（原子操作）。
// KEYS: [open_key, until_key, probe_key]
// ARGV: [now_unix, probe_ttl_sec]
// 返回: "closed" | "open" | "half_open_granted" | "half_open_denied"
var circuitBreakerCheckScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local probe_ttl = tonumber(ARGV[2])
local open_exists = redis.call('EXISTS', KEYS[1])
if open_exists == 1 then
	return 'open'
end

local until_raw = redis.call('GET', KEYS[2])
if until_raw then
	local until_ts = tonumber(until_raw)
	if until_ts and until_ts > now then
		return 'open'
	end
	local acquired = redis.call('SET', KEYS[3], '1', 'NX', 'EX', probe_ttl)
	if acquired then
		return 'half_open_granted'
	end
	return 'half_open_denied'
end

return 'closed'
`)

// circuitBreakerRecordFailureScript 记录失败并按阈值触发熔断的 Lua 脚本。
// KEYS: [model_fails, model_open, model_until, model_probe, upstream_fails, upstream_open, upstream_until, upstream_probe]
// ARGV: [now, uid, model_window_sec, model_threshold, model_duration_sec,
//
//	upstream_window_sec, upstream_fail_threshold, upstream_model_threshold,
//	upstream_logic, upstream_duration_sec, active_model_keys_json, probe_ttl]
var circuitBreakerRecordFailureScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local uid = ARGV[2]
local model_window = tonumber(ARGV[3])
local model_threshold = tonumber(ARGV[4])
local model_duration = tonumber(ARGV[5])
local upstream_window = tonumber(ARGV[6])
local upstream_fail_threshold = tonumber(ARGV[7])
local upstream_model_threshold = tonumber(ARGV[8])
local upstream_logic = ARGV[9]
local upstream_duration = tonumber(ARGV[10])
local active_model_keys = cjson.decode(ARGV[11])
local probe_ttl = tonumber(ARGV[12])

local model_probe_exists = redis.call('EXISTS', KEYS[4]) == 1
redis.call('DEL', KEYS[4])

redis.call('ZADD', KEYS[1], now, uid)
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - model_window)
redis.call('EXPIRE', KEYS[1], model_window + 60)

local model_fail_count = redis.call('ZCARD', KEYS[1])
local model_should_trip = model_probe_exists
if model_threshold > 0 and model_fail_count >= model_threshold then
	model_should_trip = true
end

local model_tripped = 0
if model_should_trip and model_duration > 0 then
	redis.call('SET', KEYS[2], '1', 'EX', model_duration)
	redis.call('SET', KEYS[3], tostring(now + model_duration), 'EX', model_duration + probe_ttl)
	model_tripped = 1
end

local upstream_probe_exists = redis.call('EXISTS', KEYS[8]) == 1
redis.call('DEL', KEYS[8])

redis.call('ZADD', KEYS[5], now, uid)
redis.call('ZREMRANGEBYSCORE', KEYS[5], '-inf', now - upstream_window)
redis.call('EXPIRE', KEYS[5], upstream_window + 60)

local fail_met = upstream_probe_exists
if upstream_fail_threshold > 0 then
	local upstream_fail_count = redis.call('ZCARD', KEYS[5])
	if upstream_fail_count >= upstream_fail_threshold then
		fail_met = true
	end
end

local model_met = false
if upstream_model_threshold > 0 then
	local circuited = 0
	for _, model_key in ipairs(active_model_keys) do
		local key = string.gsub(KEYS[2], ':[^:]+:open$', ':' .. model_key .. ':open')
		if redis.call('EXISTS', key) == 1 then
			circuited = circuited + 1
		end
	end
	model_met = (circuited >= upstream_model_threshold)
end

local upstream_should_trip = false
if upstream_logic == 'and' then
	if upstream_fail_threshold > 0 and upstream_model_threshold > 0 then
		upstream_should_trip = fail_met and model_met
	elseif upstream_fail_threshold > 0 then
		upstream_should_trip = fail_met
	elseif upstream_model_threshold > 0 then
		upstream_should_trip = model_met
	end
else
	upstream_should_trip = fail_met or model_met
end

local upstream_tripped = 0
if upstream_should_trip and upstream_duration > 0 then
	redis.call('SET', KEYS[6], '1', 'EX', upstream_duration)
	redis.call('SET', KEYS[7], tostring(now + upstream_duration), 'EX', upstream_duration + probe_ttl)
	upstream_tripped = 1
end

return cjson.encode({ model_tripped = model_tripped, upstream_tripped = upstream_tripped })
`)

// channelCache 实现 repository.ChannelCacheRepository。
type channelCache struct {
	client *redis.Client
}

// NewChannelCache 创建 ChannelCacheRepository 实现。
func NewChannelCache(client *redis.Client) repository.ChannelCacheRepository {
	return &channelCache{client: client}
}

// ---------------------------------------------------------------------------
// 熔断状态检查
// ---------------------------------------------------------------------------

// CheckUpstreamCircuitState 检查上游级熔断状态。
func (c *channelCache) CheckUpstreamCircuitState(ctx context.Context, upstreamID uint) (string, error) {
	if c.client == nil || upstreamID == 0 {
		return "closed", nil
	}
	return c.checkCircuitState(ctx,
		cbUpstreamOpenKey(upstreamID),
		cbUpstreamUntilKey(upstreamID),
		cbUpstreamProbeKey(upstreamID),
	)
}

// CheckModelCircuitState 检查模型级熔断状态。
func (c *channelCache) CheckModelCircuitState(ctx context.Context, upstreamID uint, modelKey string) (string, error) {
	if c.client == nil || upstreamID == 0 || strings.TrimSpace(modelKey) == "" {
		return "closed", nil
	}
	return c.checkCircuitState(ctx,
		cbModelOpenKey(upstreamID, modelKey),
		cbModelUntilKey(upstreamID, modelKey),
		cbModelProbeKey(upstreamID, modelKey),
	)
}

func (c *channelCache) checkCircuitState(ctx context.Context, openKey, untilKey, probeKey string) (string, error) {
	result, err := circuitBreakerCheckScript.Run(ctx, c.client, []string{openKey, untilKey, probeKey},
		time.Now().UTC().Unix(), circuitProbeTTLSec).Result()
	if err != nil {
		return "", err
	}
	state, ok := result.(string)
	if !ok || strings.TrimSpace(state) == "" {
		return "", fmt.Errorf("unexpected circuit check result")
	}
	return state, nil
}

// ---------------------------------------------------------------------------
// 熔断失败记录
// ---------------------------------------------------------------------------

// RecordCircuitFailure 使用 Lua 脚本原子记录失败并按阈值触发熔断。
func (c *channelCache) RecordCircuitFailure(ctx context.Context, input repository.CircuitFailureInput) error {
	if c.client == nil || input.UpstreamID == 0 || strings.TrimSpace(input.ModelKey) == "" {
		return nil
	}
	activeModelKeysJSON, err := json.Marshal(input.ActiveModelKeys)
	if err != nil {
		return err
	}
	_, err = circuitBreakerRecordFailureScript.Run(ctx, c.client, []string{
		cbModelFailsKey(input.UpstreamID, input.ModelKey),
		cbModelOpenKey(input.UpstreamID, input.ModelKey),
		cbModelUntilKey(input.UpstreamID, input.ModelKey),
		cbModelProbeKey(input.UpstreamID, input.ModelKey),
		cbUpstreamFailsKey(input.UpstreamID),
		cbUpstreamOpenKey(input.UpstreamID),
		cbUpstreamUntilKey(input.UpstreamID),
		cbUpstreamProbeKey(input.UpstreamID),
	},
		time.Now().UTC().Unix(),
		uuid.NewString(),
		input.ModelWindowSec,
		input.ModelFailureThreshold,
		input.ModelDurationSec,
		input.UpstreamWindowSec,
		input.UpstreamFailureThreshold,
		input.UpstreamModelThreshold,
		input.UpstreamThresholdLogic,
		input.UpstreamDurationSec,
		string(activeModelKeysJSON),
		circuitProbeTTLSec,
	).Result()
	return err
}

// RecordFailureMetadata 记录上游最近失败时间与错误信息（非关键路径）。
func (c *channelCache) RecordFailureMetadata(ctx context.Context, upstreamID uint, lastError string) {
	if c.client == nil || upstreamID == 0 {
		return
	}
	_ = c.client.Set(ctx, cbUpstreamLastFailureAtKey(upstreamID), time.Now().UTC().Format(time.RFC3339), 10*time.Minute).Err()
	if strings.TrimSpace(lastError) != "" {
		_ = c.client.Set(ctx, cbUpstreamLastErrorKey(upstreamID), lastError, 10*time.Minute).Err()
	}
}

// ---------------------------------------------------------------------------
// 熔断成功处理
// ---------------------------------------------------------------------------

// RecordSuccessMetadata 记录上游最近成功时间。
func (c *channelCache) RecordSuccessMetadata(ctx context.Context, upstreamID uint) {
	if c.client == nil || upstreamID == 0 {
		return
	}
	_ = c.client.Set(ctx, cbUpstreamLastSuccessAtKey(upstreamID), time.Now().UTC().Format(time.RFC3339), 24*time.Hour).Err()
}

// ClearUpstreamCircuitKeys 清除 probe 成功后的上游熔断关键键。
func (c *channelCache) ClearUpstreamCircuitKeys(ctx context.Context, upstreamID uint) error {
	if c.client == nil || upstreamID == 0 {
		return nil
	}
	return c.client.Del(ctx,
		cbUpstreamFailsKey(upstreamID),
		cbUpstreamOpenKey(upstreamID),
		cbUpstreamUntilKey(upstreamID),
		cbUpstreamProbeKey(upstreamID),
	).Err()
}

// ClearModelCircuitKeys 清除 probe 成功后的模型级熔断关键键。
func (c *channelCache) ClearModelCircuitKeys(ctx context.Context, upstreamID uint, modelKey string) error {
	if c.client == nil || upstreamID == 0 || strings.TrimSpace(modelKey) == "" {
		return nil
	}
	return c.client.Del(ctx,
		cbModelFailsKey(upstreamID, modelKey),
		cbModelOpenKey(upstreamID, modelKey),
		cbModelUntilKey(upstreamID, modelKey),
		cbModelProbeKey(upstreamID, modelKey),
	).Err()
}

// ReleaseRouteProbes 释放路由上的 probe 令牌。
func (c *channelCache) ReleaseRouteProbes(ctx context.Context, upstreamID uint, modelKey string) error {
	if c.client == nil || upstreamID == 0 {
		return nil
	}
	keys := []string{cbUpstreamProbeKey(upstreamID)}
	if strings.TrimSpace(modelKey) != "" {
		keys = append(keys, cbModelProbeKey(upstreamID, modelKey))
	}
	_, err := c.client.Del(ctx, keys...).Result()
	return err
}

// ---------------------------------------------------------------------------
// 手动熔断控制
// ---------------------------------------------------------------------------

// OpenUpstreamCircuit 手动打开上游熔断（24 小时）。
func (c *channelCache) OpenUpstreamCircuit(ctx context.Context, upstreamID uint) error {
	if c.client == nil || upstreamID == 0 {
		return nil
	}
	now := time.Now().UTC()
	untilTS := fmt.Sprintf("%d", now.Add(circuitManualOpenDuration).Unix())
	untilTTL := circuitManualOpenDuration + time.Duration(circuitProbeTTLSec)*time.Second
	if err := c.client.Set(ctx, cbUpstreamOpenKey(upstreamID), "1", circuitManualOpenDuration).Err(); err != nil {
		return err
	}
	return c.client.Set(ctx, cbUpstreamUntilKey(upstreamID), untilTS, untilTTL).Err()
}

// ResetUpstreamCircuit 重置上游全量熔断与计数键。
func (c *channelCache) ResetUpstreamCircuit(ctx context.Context, upstreamID uint) error {
	if c.client == nil || upstreamID == 0 {
		return nil
	}
	return c.client.Del(ctx,
		cbUpstreamOpenKey(upstreamID),
		cbUpstreamUntilKey(upstreamID),
		cbUpstreamProbeKey(upstreamID),
		cbUpstreamFailsKey(upstreamID),
		cbUpstreamLastErrorKey(upstreamID),
		cbUpstreamLastFailureAtKey(upstreamID),
		cbUpstreamLastSuccessAtKey(upstreamID),
	).Err()
}

// OpenModelCircuit 手动打开模型级熔断（24 小时）。
func (c *channelCache) OpenModelCircuit(ctx context.Context, upstreamID uint, modelKey string) error {
	if c.client == nil || upstreamID == 0 || strings.TrimSpace(modelKey) == "" {
		return nil
	}
	now := time.Now().UTC()
	untilTS := fmt.Sprintf("%d", now.Add(circuitManualOpenDuration).Unix())
	untilTTL := circuitManualOpenDuration + time.Duration(circuitProbeTTLSec)*time.Second
	if err := c.client.Set(ctx, cbModelOpenKey(upstreamID, modelKey), "1", circuitManualOpenDuration).Err(); err != nil {
		return err
	}
	return c.client.Set(ctx, cbModelUntilKey(upstreamID, modelKey), untilTS, untilTTL).Err()
}

// ResetModelCircuit 重置模型级熔断与计数键。
func (c *channelCache) ResetModelCircuit(ctx context.Context, upstreamID uint, modelKey string) error {
	if c.client == nil || upstreamID == 0 || strings.TrimSpace(modelKey) == "" {
		return nil
	}
	return c.client.Del(ctx,
		cbModelOpenKey(upstreamID, modelKey),
		cbModelUntilKey(upstreamID, modelKey),
		cbModelProbeKey(upstreamID, modelKey),
		cbModelFailsKey(upstreamID, modelKey),
	).Err()
}

// ---------------------------------------------------------------------------
// 状态查询（列表展示用）
// ---------------------------------------------------------------------------

// QueryUpstreamCircuitStatus 查询上游熔断展示状态。
func (c *channelCache) QueryUpstreamCircuitStatus(ctx context.Context, upstreamID uint) (open bool, until string) {
	if c.client == nil || upstreamID == 0 {
		return false, ""
	}
	return c.queryCircuitStatus(ctx, cbUpstreamOpenKey(upstreamID), cbUpstreamUntilKey(upstreamID))
}

// QueryModelCircuitStatus 查询模型级熔断展示状态。
func (c *channelCache) QueryModelCircuitStatus(ctx context.Context, upstreamID uint, modelKey string) (open bool, until string) {
	if c.client == nil || upstreamID == 0 || strings.TrimSpace(modelKey) == "" {
		return false, ""
	}
	return c.queryCircuitStatus(ctx, cbModelOpenKey(upstreamID, modelKey), cbModelUntilKey(upstreamID, modelKey))
}

func (c *channelCache) queryCircuitStatus(ctx context.Context, openKey, untilKey string) (bool, string) {
	n, err := c.client.Exists(ctx, openKey).Result()
	if err != nil || n == 0 {
		return false, ""
	}
	until, _ := c.client.Get(ctx, untilKey).Result()
	return true, until
}

// ---------------------------------------------------------------------------
// 限流状态
// ---------------------------------------------------------------------------

// IsRateLimited 判断上游当前是否处于 rate limit 退避中。
func (c *channelCache) IsRateLimited(ctx context.Context, upstreamID uint) bool {
	if c.client == nil || upstreamID == 0 {
		return false
	}
	v, err := c.client.Get(ctx, rateLimitBackoffKey(upstreamID)).Result()
	return err == nil && strings.TrimSpace(v) != ""
}

// RecordRateLimitBackoff 根据指数退避参数记录退避状态。
func (c *channelCache) RecordRateLimitBackoff(ctx context.Context, upstreamID uint, params repository.RateLimitBackoffParams) error {
	if c.client == nil || upstreamID == 0 {
		return nil
	}
	backoffCount, err := c.client.Incr(ctx, rateLimitBackoffCountKey(upstreamID)).Result()
	if err != nil {
		return err
	}
	backoffSec := calculateBackoffSeconds(backoffCount, params)
	if backoffSec <= 0 {
		return nil
	}
	_ = c.client.Expire(ctx, rateLimitBackoffCountKey(upstreamID), 5*time.Minute).Err()
	untilTS := fmt.Sprintf("%d", time.Now().UTC().Add(time.Duration(backoffSec)*time.Second).Unix())
	_ = c.client.Set(ctx, rateLimitBackoffKey(upstreamID), "1", time.Duration(backoffSec)*time.Second).Err()
	_ = c.client.Set(ctx, rateLimitBackoffUntilKey(upstreamID), untilTS, time.Duration(backoffSec)*time.Second).Err()
	return nil
}

func calculateBackoffSeconds(attempt int64, params repository.RateLimitBackoffParams) int {
	base := params.BackoffBaseSec
	if base <= 0 {
		base = 5
	}
	maxSec := params.BackoffMaxSec
	if maxSec <= 0 {
		maxSec = 60
	}
	multiplier := params.BackoffMultiplier
	if multiplier <= 1 {
		multiplier = 2
	}
	backoff := base
	for i := int64(1); i < attempt; i++ {
		if backoff >= maxSec {
			return maxSec
		}
		backoff *= multiplier
		if backoff >= maxSec {
			return maxSec
		}
	}
	return backoff
}

// ---------------------------------------------------------------------------
// API Key 轮询计数
// ---------------------------------------------------------------------------

// IncrAPIKeyCounter 原子递增 API Key 轮询计数器。
func (c *channelCache) IncrAPIKeyCounter(ctx context.Context, upstreamID uint) (int64, bool) {
	if c.client == nil || upstreamID == 0 {
		return 0, false
	}
	next, err := c.client.Incr(ctx, apiKeyCounterKey(upstreamID)).Result()
	if err != nil || next <= 0 {
		return 0, false
	}
	return next - 1, true
}

// ---------------------------------------------------------------------------
// Redis Key 构造
// ---------------------------------------------------------------------------

func cbUpstreamOpenKey(id uint) string          { return fmt.Sprintf("cb:u:%d:open", id) }
func cbUpstreamFailsKey(id uint) string         { return fmt.Sprintf("cb:u:%d:fails", id) }
func cbUpstreamUntilKey(id uint) string         { return fmt.Sprintf("cb:u:%d:until", id) }
func cbUpstreamProbeKey(id uint) string         { return fmt.Sprintf("cb:u:%d:probe", id) }
func cbUpstreamLastErrorKey(id uint) string     { return fmt.Sprintf("cb:u:%d:last_error", id) }
func cbUpstreamLastFailureAtKey(id uint) string { return fmt.Sprintf("cb:u:%d:last_failure_at", id) }
func cbUpstreamLastSuccessAtKey(id uint) string { return fmt.Sprintf("cb:u:%d:last_success_at", id) }

func cbModelOpenKey(upstreamID uint, modelKey string) string {
	return fmt.Sprintf("cb:u:%d:m:%s:open", upstreamID, modelKey)
}
func cbModelFailsKey(upstreamID uint, modelKey string) string {
	return fmt.Sprintf("cb:u:%d:m:%s:fails", upstreamID, modelKey)
}
func cbModelUntilKey(upstreamID uint, modelKey string) string {
	return fmt.Sprintf("cb:u:%d:m:%s:until", upstreamID, modelKey)
}
func cbModelProbeKey(upstreamID uint, modelKey string) string {
	return fmt.Sprintf("cb:u:%d:m:%s:probe", upstreamID, modelKey)
}

func rateLimitBackoffKey(id uint) string      { return fmt.Sprintf("rl:u:%d:backoff", id) }
func rateLimitBackoffUntilKey(id uint) string { return fmt.Sprintf("rl:u:%d:backoff_until", id) }
func rateLimitBackoffCountKey(id uint) string { return fmt.Sprintf("rl:u:%d:backoff_count", id) }
func apiKeyCounterKey(id uint) string         { return fmt.Sprintf("llm:u:%d:key_counter", id) }
