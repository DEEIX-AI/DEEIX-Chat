package channel

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// 路由解析：权重随机负载均衡 + 上游/模型两级熔断
// ---------------------------------------------------------------------------

// ResolveRoute 解析模型路由，应用权重随机负载均衡与两级熔断过滤。
func (s *Service) ResolveRoute(ctx context.Context, input ResolveRouteInput) (*ResolvedRoute, error) {
	breakerEnabled := s.cache != nil && s.loadBreakerDefaults(ctx).Enabled
	platformModelName, err := normalizePlatformModelName(input.PlatformModelName)
	if err != nil {
		return nil, ErrModelNotFound
	}
	platformModel, err := s.repo.GetActiveModelByName(ctx, platformModelName)
	if err != nil {
		return nil, err
	}
	if !routeScopeAllowsModelAccess(input.Scope, platformModel.AccessScope) {
		return nil, ErrModelAccessDenied
	}
	if normalizeRouteScope(input.Scope) == RouteScopeUser && input.UserID > 0 {
		accessible, err := s.isModelAccessible(ctx, platformModel.ID, input.UserID)
		if err != nil {
			return nil, err
		}
		if !accessible {
			return nil, ErrModelAccessDenied
		}
	}

	rows, err := s.repo.ListActiveRoutesByModel(ctx, platformModelName)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrRouteNotFound
	}

	excludedRouteIDs := makeRouteIDSet(input.ExcludedRouteIDs)
	available := make([]repository.ChannelUpstreamRouteRow, 0, len(rows))
	for _, row := range rows {
		if _, excluded := excludedRouteIDs[row.RouteID]; excluded {
			continue
		}
		if !IsRouteAllowedForTask(input.TaskType, row.ModelKindsJSON, row.Protocol) {
			continue
		}
		available = append(available, row)
	}
	if len(available) == 0 {
		return nil, ErrAllRoutesUnavailable
	}

	var shortestRateLimitBackoff time.Duration
	for start := 0; start < len(available); {
		priority := available[start].RoutePriority
		group := make([]routeCandidate, 0, 4)

		for start < len(available) && available[start].RoutePriority == priority {
			row := available[start]
			start++

			if row.UpstreamModelID == 0 || strings.TrimSpace(row.BindingCode) == "" || strings.TrimSpace(row.UpstreamModelName) == "" {
				continue
			}
			if !llm.IsImplementedAdapter(row.Protocol) {
				continue
			}
			if err := s.validateUpstreamBaseURL(row.BaseURL); err != nil {
				s.warn("unsafe_upstream_base_url_skipped", zap.Uint("upstream_id", row.UpstreamID), zap.Error(err))
				continue
			}
			if remaining := s.routeRateLimitBackoff(ctx, row.UpstreamID, row.RouteID); remaining > 0 {
				if shortestRateLimitBackoff == 0 || remaining < shortestRateLimitBackoff {
					shortestRateLimitBackoff = remaining
				}
				continue
			}

			keyCfg, keyErr := s.parseAPIKeysConfig(row.APIKeysEnc)
			if keyErr != nil {
				continue
			}
			apiKey, keyErr := s.selectAPIKey(ctx, row.UpstreamID, keyCfg)
			if keyErr != nil {
				continue
			}
			group = append(group, routeCandidate{row: row, apiKey: apiKey})
		}

		if len(group) == 0 {
			continue
		}

		candidates := append([]routeCandidate(nil), group...)
		for len(candidates) > 0 {
			selected := weightedRandomCandidate(candidates)
			if selected == nil {
				selected = &candidates[0]
			}

			upstreamState := "closed"
			modelState := "closed"
			if breakerEnabled {
				upstreamState, err = s.checkUpstreamCircuitState(ctx, selected.row.UpstreamID)
				if err != nil {
					return nil, err
				}
				if upstreamState == "open" || upstreamState == "half_open_denied" {
					candidates = removeCandidate(candidates, selected.row.UpstreamID, selected.row.UpstreamModelID)
					continue
				}

				modelCircuitKey := bindingCircuitKey(selected.row.BindingCode)
				modelState, err = s.checkModelCircuitState(ctx, selected.row.UpstreamID, modelCircuitKey)
				if err != nil {
					if upstreamState == "half_open_granted" {
						s.releaseUpstreamProbe(ctx, selected.row.UpstreamID)
					}
					return nil, err
				}
				if modelState == "open" || modelState == "half_open_denied" {
					if upstreamState == "half_open_granted" {
						s.releaseUpstreamProbe(ctx, selected.row.UpstreamID)
					}
					candidates = removeCandidate(candidates, selected.row.UpstreamID, selected.row.UpstreamModelID)
					continue
				}
			}

			resolved := buildResolvedRoute(selected.row, selected.apiKey)
			resolved.UpstreamProbeGranted = upstreamState == "half_open_granted"
			resolved.ModelProbeGranted = modelState == "half_open_granted"
			return resolved, nil
		}
	}

	if shortestRateLimitBackoff > 0 {
		return nil, &RoutesRateLimitedError{RetryAfter: shortestRateLimitBackoff}
	}
	return nil, ErrAllRoutesUnavailable
}

func makeRouteIDSet(routeIDs []uint) map[uint]struct{} {
	if len(routeIDs) == 0 {
		return nil
	}
	result := make(map[uint]struct{}, len(routeIDs))
	for _, routeID := range routeIDs {
		if routeID != 0 {
			result[routeID] = struct{}{}
		}
	}
	return result
}

func normalizeRouteScope(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case RouteScopeInternal:
		return RouteScopeInternal
	default:
		return RouteScopeUser
	}
}

func routeScopeAllowsModelAccess(routeScope string, modelAccessScope string) bool {
	if normalizeRouteScope(routeScope) == RouteScopeInternal {
		return true
	}
	return normalizeModelAccessScopeValue(modelAccessScope) == ModelAccessScopePublic
}

// MarkRouteSuccess 标记上游调用成功，清除失败计数。
func (s *Service) MarkRouteSuccess(ctx context.Context, route *ResolvedRoute) {
	if route == nil || route.UpstreamID == 0 || s.cache == nil {
		return
	}
	metaCtx := bookkeepingContext(ctx)
	if err := s.cache.ClearRateLimitBackoff(metaCtx, route.UpstreamID, route.RouteID); err != nil {
		s.warn("clear_route_rate_limit_backoff_failed",
			zap.Uint("upstream_id", route.UpstreamID),
			zap.Uint("route_id", route.RouteID),
			zap.Error(err),
		)
	}
	if route.UpstreamProbeGranted {
		if err := s.cache.ClearUpstreamCircuitKeys(metaCtx, route.UpstreamID); err != nil {
			s.warn("clear_upstream_circuit_keys_failed", zap.Uint("upstream_id", route.UpstreamID), zap.Error(err))
		}
	}
	modelCircuitKey := routeModelCircuitKey(route)
	if route.ModelProbeGranted && modelCircuitKey != "" {
		if err := s.cache.ClearModelCircuitKeys(metaCtx, route.UpstreamID, modelCircuitKey); err != nil {
			s.warn("clear_model_circuit_keys_failed",
				zap.Uint("upstream_id", route.UpstreamID),
				zap.Uint("upstream_model_id", route.UpstreamModelID),
				zap.Error(err),
			)
		}
	}
	s.cache.RecordSuccessMetadata(metaCtx, route.UpstreamID)
}

// MarkRouteFailure 标记上游调用失败，按照错误分类执行熔断或退避。
func (s *Service) MarkRouteFailure(ctx context.Context, route *ResolvedRoute, cause error) {
	if route == nil || route.UpstreamID == 0 || s.cache == nil {
		return
	}

	metaCtx := bookkeepingContext(ctx)
	lastErrMsg := truncateMessage(strings.TrimSpace(errorMessage(cause)), 255)
	s.cache.RecordFailureMetadata(metaCtx, route.UpstreamID, lastErrMsg)

	switch s.classifyRouteFailure(metaCtx, cause) {
	case routeFailureIgnore:
		s.releaseGrantedRouteProbes(metaCtx, route)
		return
	case routeFailureRateLimit:
		s.releaseGrantedRouteProbes(metaCtx, route)
		s.recordRateLimitBackoff(metaCtx, route, cause)
	default:
		defaults := s.loadBreakerDefaults(metaCtx)
		if !defaults.Enabled {
			s.releaseGrantedRouteProbes(metaCtx, route)
			return
		}
		s.recordCircuitFailure(metaCtx, route, defaults)
	}
}

// ---------------------------------------------------------------------------
// 熔断辅助
// ---------------------------------------------------------------------------

// bookkeepingContext 返回一个不受请求取消影响的 context，用于后台计量写入。
func bookkeepingContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return context.WithoutCancel(ctx)
}

func (s *Service) recordCircuitFailure(ctx context.Context, route *ResolvedRoute, defaults domainchannel.BreakerDefaults) {
	modelCircuitKey := routeModelCircuitKey(route)
	if route.UpstreamID == 0 || modelCircuitKey == "" {
		return
	}

	platformModelPolicyEnforced := normalizeModelCircuitPolicyMode(route.PlatformModelCbPolicyMode) == "enforced"
	resolveModelCircuitValue := func(routeValue int, platformValue int, defaultValue int) int {
		if platformModelPolicyEnforced || routeValue <= 0 {
			routeValue = platformValue
		}
		if routeValue <= 0 {
			routeValue = defaultValue
		}
		return routeValue
	}
	modelThreshold := resolveModelCircuitValue(route.ModelCbFailureThreshold, route.PlatformModelCbFailureThreshold, defaults.ModelFailureThreshold)
	modelWindowMin := resolveModelCircuitValue(route.ModelCbWindowMin, route.PlatformModelCbWindowMin, defaults.ModelWindowMin)
	modelDurationMin := resolveModelCircuitValue(route.ModelCbDurationMin, route.PlatformModelCbDurationMin, defaults.ModelDurationMin)

	upstreamFailureThreshold := route.UpstreamCbFailureThreshold
	if upstreamFailureThreshold <= 0 {
		upstreamFailureThreshold = defaults.UpstreamFailureThreshold
	}
	upstreamModelThreshold := route.UpstreamCbModelThreshold
	if upstreamModelThreshold <= 0 {
		upstreamModelThreshold = defaults.UpstreamModelThreshold
	}
	upstreamWindowMin := route.UpstreamCbWindowMin
	if upstreamWindowMin <= 0 {
		upstreamWindowMin = defaults.UpstreamWindowMin
	}
	upstreamDurationMin := route.UpstreamCbDurationMin
	if upstreamDurationMin <= 0 {
		upstreamDurationMin = defaults.UpstreamDurationMin
	}
	upstreamLogic := normalizeCbLogic(route.UpstreamCbThresholdLogic)
	if strings.TrimSpace(route.UpstreamCbThresholdLogic) == "" {
		upstreamLogic = normalizeCbLogic(defaults.UpstreamThresholdLogic)
	}

	activeBindingCodes, err := s.repo.ListActiveRouteBindingCodesForUpstream(ctx, route.UpstreamID)
	activeModelKeys := bindingCircuitKeys(activeBindingCodes)
	if err != nil || len(activeModelKeys) == 0 {
		activeModelKeys = []string{modelCircuitKey}
	}

	if err := s.cache.RecordCircuitFailure(ctx, repository.CircuitFailureInput{
		UpstreamID:               route.UpstreamID,
		ModelKey:                 modelCircuitKey,
		ModelWindowSec:           modelWindowMin * 60,
		ModelFailureThreshold:    modelThreshold,
		ModelDurationSec:         modelDurationMin * 60,
		UpstreamWindowSec:        upstreamWindowMin * 60,
		UpstreamFailureThreshold: upstreamFailureThreshold,
		UpstreamModelThreshold:   upstreamModelThreshold,
		UpstreamThresholdLogic:   upstreamLogic,
		UpstreamDurationSec:      upstreamDurationMin * 60,
		ActiveModelKeys:          activeModelKeys,
	}); err != nil {
		s.warn("record_circuit_failure_failed",
			zap.Uint("upstream_id", route.UpstreamID),
			zap.Uint("upstream_model_id", route.UpstreamModelID),
			zap.Error(err),
		)
	}
}

func (s *Service) routeRateLimitBackoff(ctx context.Context, upstreamID uint, routeID uint) time.Duration {
	if s.cache == nil || upstreamID == 0 || routeID == 0 {
		return 0
	}
	remaining, err := s.cache.GetRateLimitBackoff(ctx, upstreamID, routeID)
	if err != nil {
		s.warn("get_route_rate_limit_backoff_failed",
			zap.Uint("upstream_id", upstreamID),
			zap.Uint("route_id", routeID),
			zap.Error(err),
		)
		return 0
	}
	return remaining
}

func (s *Service) checkUpstreamCircuitState(ctx context.Context, upstreamID uint) (string, error) {
	return s.cache.CheckUpstreamCircuitState(ctx, upstreamID)
}

func (s *Service) checkModelCircuitState(ctx context.Context, upstreamID uint, modelKey string) (string, error) {
	return s.cache.CheckModelCircuitState(ctx, upstreamID, modelKey)
}

func (s *Service) releaseUpstreamProbe(ctx context.Context, upstreamID uint) {
	if err := s.cache.ReleaseRouteProbes(ctx, upstreamID, ""); err != nil {
		s.warn("release_upstream_probe_failed", zap.Uint("upstream_id", upstreamID), zap.Error(err))
	}
}

func (s *Service) releaseGrantedRouteProbes(ctx context.Context, route *ResolvedRoute) {
	if route == nil || route.UpstreamID == 0 {
		return
	}
	if route.UpstreamProbeGranted {
		if err := s.cache.ReleaseRouteProbes(ctx, route.UpstreamID, ""); err != nil {
			s.warn("release_route_probe_failed", zap.Uint("upstream_id", route.UpstreamID), zap.Error(err))
		}
	}
	if route.ModelProbeGranted {
		modelKey := routeModelCircuitKey(route)
		if modelKey == "" {
			return
		}
		if err := s.cache.ReleaseRouteProbes(ctx, route.UpstreamID, modelKey); err != nil {
			s.warn("release_model_route_probe_failed",
				zap.Uint("upstream_id", route.UpstreamID),
				zap.Uint("upstream_model_id", route.UpstreamModelID),
				zap.Error(err),
			)
		}
	}
}

func removeCandidate(items []routeCandidate, upstreamID uint, upstreamModelID uint) []routeCandidate {
	filtered := items[:0]
	for _, item := range items {
		if item.row.UpstreamID == upstreamID && item.row.UpstreamModelID == upstreamModelID {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

// ---------------------------------------------------------------------------
// 路由构造辅助
// ---------------------------------------------------------------------------

func buildResolvedRoute(row repository.ChannelUpstreamRouteRow, apiKey string) *ResolvedRoute {
	route := &ResolvedRoute{
		RouteID:                         row.RouteID,
		PlatformModelID:                 row.PlatformModelID,
		PlatformModelName:               strings.TrimSpace(row.PlatformModelName),
		UpstreamModelID:                 row.UpstreamModelID,
		UpstreamID:                      row.UpstreamID,
		UpstreamName:                    strings.TrimSpace(row.UpstreamName),
		BindingCode:                     strings.TrimSpace(row.BindingCode),
		Protocol:                        row.Protocol,
		BaseURL:                         strings.TrimSpace(row.BaseURL),
		APIKey:                          apiKey,
		ConnectTimeoutMS:                row.ConnectTimeoutMS,
		ReadTimeoutMS:                   row.ReadTimeoutMS,
		StreamIdleTimeoutMS:             row.StreamIdleTimeoutMS,
		HeadersJSON:                     mergeHeaderJSON(row.HeadersJSON, row.RouteHeadersJSON),
		ModelVendor:                     strings.TrimSpace(row.ModelVendor),
		ModelIcon:                       strings.TrimSpace(row.ModelIcon),
		ModelCapabilitiesJSON:           strings.TrimSpace(row.ModelCapabilitiesJSON),
		ModelSystemPrompt:               strings.TrimSpace(row.ModelSystemPrompt),
		UpstreamModel:                   strings.TrimSpace(row.UpstreamModelName),
		ReasoningContentPassback:        reasoningContentPassbackRequired(row.Protocol, row.ModelVendor, row.PlatformModelName, row.UpstreamModelName, row.UpstreamName),
		ReasoningPassbackRequestOptions: reasoningPassbackRequestOptions(row.Protocol, row.ModelVendor, row.PlatformModelName, row.UpstreamModelName, row.UpstreamName),
		UpstreamCbFailureThreshold:      row.UpstreamCbFailureThreshold,
		UpstreamCbModelThreshold:        row.UpstreamCbModelThreshold,
		UpstreamCbThresholdLogic:        row.UpstreamCbThresholdLogic,
		UpstreamCbDurationMin:           row.UpstreamCbDurationMin,
		UpstreamCbWindowMin:             row.UpstreamCbWindowMin,
		PlatformModelCbPolicyMode:       row.PlatformModelCbPolicyMode,
		PlatformModelCbFailureThreshold: row.PlatformModelCbFailureThreshold,
		PlatformModelCbDurationMin:      row.PlatformModelCbDurationMin,
		PlatformModelCbWindowMin:        row.PlatformModelCbWindowMin,
		ModelCbFailureThreshold:         row.ModelCbFailureThreshold,
		ModelCbDurationMin:              row.ModelCbDurationMin,
		ModelCbWindowMin:                row.ModelCbWindowMin,
	}
	return route
}

// bindingCircuitKey 返回上游真实模型绑定级熔断 key。
func bindingCircuitKey(bindingCode string) string {
	code := strings.TrimSpace(bindingCode)
	if code == "" {
		return ""
	}
	return "upstream-model-" + strings.ReplaceAll(code, ":", "-")
}

func bindingCircuitKeys(bindingCodes []string) []string {
	keys := make([]string, 0, len(bindingCodes))
	for _, code := range bindingCodes {
		key := bindingCircuitKey(code)
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func routeModelCircuitKey(route *ResolvedRoute) string {
	if route == nil {
		return ""
	}
	return bindingCircuitKey(route.BindingCode)
}

func weightedRandomCandidate(rows []routeCandidate) *routeCandidate {
	if len(rows) == 0 {
		return nil
	}
	totalWeight := 0
	for _, row := range rows {
		w := row.row.Weight
		if w <= 0 {
			w = 100
		}
		totalWeight += w
	}
	if totalWeight <= 0 {
		idx := rand.Intn(len(rows))
		return &rows[idx]
	}
	r := rand.Intn(totalWeight)
	cumulative := 0
	for i, row := range rows {
		w := row.row.Weight
		if w <= 0 {
			w = 100
		}
		cumulative += w
		if r < cumulative {
			return &rows[i]
		}
	}
	return &rows[len(rows)-1]
}

// ---------------------------------------------------------------------------
// 多密钥选择
// ---------------------------------------------------------------------------

// selectAPIKey 从路由的 APIKeysConfig 中按策略选取一个可用 API 密钥。
func (s *Service) selectAPIKey(ctx context.Context, upstreamID uint, cfg domainchannel.APIKeysConfig) (string, error) {
	if len(cfg.Keys) == 0 {
		return "", ErrNoActiveKey
	}
	return s.pickActiveKey(ctx, upstreamID, cfg.Keys, cfg.Strategy)
}

// pickActiveKey 从有效密钥列表中按策略选取一个密钥。
func (s *Service) pickActiveKey(ctx context.Context, upstreamID uint, entries []domainchannel.APIKey, strategy string) (string, error) {
	active := make([]domainchannel.APIKey, 0, len(entries))
	for _, e := range entries {
		if (e.Status == "" || e.Status == "active") && strings.TrimSpace(e.Key) != "" {
			active = append(active, e)
		}
	}
	if len(active) == 0 {
		return "", ErrNoActiveKey
	}

	switch strings.TrimSpace(strategy) {
	case "failover":
		return active[0].Key, nil
	case "round_robin":
		idx := s.nextAPIKeyIndex(ctx, upstreamID)
		return active[int(idx%uint64(len(active)))].Key, nil
	default:
		return active[rand.Intn(len(active))].Key, nil
	}
}

func (s *Service) nextAPIKeyIndex(ctx context.Context, upstreamID uint) uint64 {
	if upstreamID == 0 {
		return uint64(rand.Intn(1024))
	}
	if s.cache != nil {
		if idx, ok := s.cache.IncrAPIKeyCounter(ctx, upstreamID); ok {
			return uint64(idx)
		}
	}
	return nextLocalAPIKeyIndex(upstreamID)
}

func nextLocalAPIKeyIndex(upstreamID uint) uint64 {
	counter, _ := localAPIKeyCounters.LoadOrStore(upstreamID, &atomic.Uint64{})
	return counter.(*atomic.Uint64).Add(1) - 1
}

// ---------------------------------------------------------------------------
// 失败分类
// ---------------------------------------------------------------------------

func (s *Service) classifyRouteFailure(ctx context.Context, cause error) routeFailureClass {
	if cause == nil {
		return routeFailureIgnore
	}
	if errors.Is(cause, context.Canceled) {
		return routeFailureIgnore
	}

	rules := s.loadBreakerErrorClassification(ctx)

	if isRateLimitFailure(cause) && matchesFailureRule(rules.RateLimitErrors, "429") {
		return routeFailureRateLimit
	}
	if isIgnorableFailure(cause) && matchesFailureRule(rules.IgnoreErrors, "4xx") {
		return routeFailureIgnore
	}
	if isCircuitFailure(cause) && (matchesFailureRule(rules.CircuitErrors, "5xx") ||
		matchesFailureRule(rules.CircuitErrors, "timeout") ||
		matchesFailureRule(rules.CircuitErrors, "connection_error")) {
		return routeFailureCircuit
	}
	return routeFailureCircuit
}

func isRateLimitFailure(cause error) bool {
	var upstreamErr *llm.UpstreamError
	return errors.As(cause, &upstreamErr) && upstreamErr.StatusCode == 429
}

func isIgnorableFailure(cause error) bool {
	var upstreamErr *llm.UpstreamError
	return errors.As(cause, &upstreamErr) && upstreamErr.StatusCode >= 400 && upstreamErr.StatusCode < 500 && upstreamErr.StatusCode != 429
}

func isCircuitFailure(cause error) bool {
	var upstreamErr *llm.UpstreamError
	if errors.As(cause, &upstreamErr) {
		return upstreamErr.StatusCode >= 500
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return true
	}
	message := strings.ToLower(errorMessage(cause))
	switch {
	case strings.Contains(message, "timeout"),
		strings.Contains(message, "deadline exceeded"),
		strings.Contains(message, "connection refused"),
		strings.Contains(message, "connection reset"),
		strings.Contains(message, "broken pipe"),
		strings.Contains(message, "dial tcp"),
		strings.Contains(message, "no such host"),
		strings.Contains(message, "eof"):
		return true
	default:
		return false
	}
}

// ShouldFailoverRoute reports whether a request can be retried on a different
// route. Validation, authorization, billing, and caller cancellation errors
// must stay on the original request path.
func ShouldFailoverRoute(cause error) bool {
	if cause == nil || errors.Is(cause, context.Canceled) || llm.RequestWasAccepted(cause) {
		return false
	}

	var upstreamErr *llm.UpstreamError
	if errors.As(cause, &upstreamErr) {
		return upstreamErr.StatusCode == http.StatusRequestTimeout ||
			upstreamErr.StatusCode == http.StatusTooManyRequests ||
			upstreamErr.StatusCode >= http.StatusInternalServerError
	}
	if errors.Is(cause, context.DeadlineExceeded) || errors.Is(cause, io.EOF) || errors.Is(cause, io.ErrUnexpectedEOF) {
		return true
	}
	var networkErr net.Error
	return errors.As(cause, &networkErr)
}

func matchesFailureRule(rules []string, target string) bool {
	normalizedTarget := strings.TrimSpace(strings.ToLower(target))
	for _, rule := range rules {
		if strings.TrimSpace(strings.ToLower(rule)) == normalizedTarget {
			return true
		}
	}
	return false
}

func (s *Service) recordRateLimitBackoff(ctx context.Context, route *ResolvedRoute, cause error) {
	if s.cache == nil || route == nil || route.UpstreamID == 0 || route.RouteID == 0 {
		return
	}
	defaults := s.loadRateLimitDefaults(ctx)
	if err := s.cache.RecordRateLimitBackoff(ctx, repository.RateLimitBackoffParams{
		UpstreamID:        route.UpstreamID,
		RouteID:           route.RouteID,
		BackoffBaseSec:    defaults.BackoffBaseSec,
		BackoffMaxSec:     defaults.BackoffMaxSec,
		BackoffMultiplier: defaults.BackoffMultiplier,
		RetryAfterSec:     upstreamRetryAfterSeconds(cause, time.Now()),
	}); err != nil {
		s.warn("record_rate_limit_backoff_failed",
			zap.Uint("upstream_id", route.UpstreamID),
			zap.Uint("route_id", route.RouteID),
			zap.Error(err),
		)
	}
}

func upstreamRetryAfterSeconds(cause error, now time.Time) int {
	var upstreamErr *llm.UpstreamError
	if !errors.As(cause, &upstreamErr) || upstreamErr.Debug == nil {
		return 0
	}
	raw := ""
	for key, value := range upstreamErr.Debug.Response.Headers {
		if strings.EqualFold(strings.TrimSpace(key), "Retry-After") {
			raw = strings.TrimSpace(value)
			break
		}
	}
	if raw == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		return max(seconds, 0)
	}
	retryAt, err := http.ParseTime(raw)
	if err != nil || !retryAt.After(now) {
		return 0
	}
	remaining := retryAt.Sub(now)
	return int((remaining + time.Second - 1) / time.Second)
}

// loadBreakerErrorClassification 从 repository 读取熔断错误分类配置（含默认值）。
func (s *Service) loadBreakerErrorClassification(ctx context.Context) domainchannel.BreakerErrorClassification {
	cfg, err := s.repo.GetBreakerErrorClassification(ctx)
	if err != nil {
		s.warn("load_breaker_error_classification_failed", zap.Error(err))
	}
	return cfg
}

// loadBreakerDefaults 读取并短时缓存熔断器默认参数，避免在模型请求热路径重复查询数据库。
// 配置缺失或首次读取失败时默认关闭；后续读取失败时保留最近一次有效配置。
func (s *Service) loadBreakerDefaults(ctx context.Context) domainchannel.BreakerDefaults {
	if s == nil || s.repo == nil {
		return domainchannel.BreakerDefaults{}
	}
	now := time.Now()
	s.breakerDefaultsMu.RLock()
	if now.Before(s.breakerDefaultsValidUntil) {
		cfg := s.breakerDefaults
		s.breakerDefaultsMu.RUnlock()
		return cfg
	}
	s.breakerDefaultsMu.RUnlock()

	s.breakerDefaultsMu.Lock()
	defer s.breakerDefaultsMu.Unlock()
	now = time.Now()
	if now.Before(s.breakerDefaultsValidUntil) {
		return s.breakerDefaults
	}
	cfg, err := s.repo.GetBreakerDefaults(ctx)
	if err != nil {
		s.warn("load_breaker_defaults_failed", zap.Error(err))
		s.breakerDefaultsValidUntil = now.Add(breakerDefaultsErrorRetryTTL)
		if s.breakerDefaultsLoaded {
			return s.breakerDefaults
		}
		return domainchannel.BreakerDefaults{}
	}
	s.breakerDefaults = cfg
	s.breakerDefaultsLoaded = true
	s.breakerDefaultsValidUntil = now.Add(breakerDefaultsCacheTTL)
	return cfg
}

// storeBreakerDefaults 让本实例立即使用已校验并成功写入的配置。
func (s *Service) storeBreakerDefaults(cfg domainchannel.BreakerDefaults) {
	if s == nil {
		return
	}
	s.breakerDefaultsMu.Lock()
	defer s.breakerDefaultsMu.Unlock()
	s.breakerDefaults = cfg
	s.breakerDefaultsLoaded = true
	s.breakerDefaultsValidUntil = time.Now().Add(breakerDefaultsCacheTTL)
}

// loadRateLimitDefaults 从 repository 读取限流退避默认参数（含默认值）。
func (s *Service) loadRateLimitDefaults(ctx context.Context) domainchannel.RateLimitDefaults {
	cfg, err := s.repo.GetRateLimitDefaults(ctx)
	if err != nil {
		s.warn("load_rate_limit_defaults_failed", zap.Error(err))
	}
	return cfg
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
