// Package outboundhttp 提供按管理员配置 origin 隔离并复用出站 HTTP 客户端的基础设施能力。
package outboundhttp

import (
	"container/list"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

const DefaultCacheLimit = 64

// ManagedClient 包装可复用 HTTP 客户端及其传输层生命周期。
type ManagedClient struct {
	Client               *http.Client
	CloseIdleConnections func()
}

// ClientFactory 根据隔离后的出站策略创建客户端。variant 用于区分连接超时等传输层配置。
type ClientFactory func(policy security.OutboundPolicy, trustedOrigin string, variant string) (ManagedClient, error)

// Pool 按精确 origin 和传输层变体维护有界 LRU 客户端池。
// basePolicy 应为严格策略；只有传入的管理员配置 endpoint 会在对应客户端中获得局部授权。
type Pool struct {
	basePolicy security.OutboundPolicy
	limit      int
	factory    ClientFactory
	entries    map[string]*list.Element
	lru        *list.List
	mu         sync.Mutex
}

type cacheEntry struct {
	key     string
	managed ManagedClient
}

// NewPool 创建按 origin 隔离的客户端池。
func NewPool(basePolicy security.OutboundPolicy, limit int, factory ClientFactory) *Pool {
	if limit <= 0 {
		limit = DefaultCacheLimit
	}
	return &Pool{
		basePolicy: basePolicy,
		limit:      limit,
		factory:    factory,
		entries:    make(map[string]*list.Element, limit),
		lru:        list.New(),
	}
}

// clientForEndpoint 返回按 configuredEndpoint origin 隔离的复用客户端。
// 精确 origin 的请求级约束由 Do 统一执行，调用方不会直接取得已扩展信任的客户端。
func (p *Pool) clientForEndpoint(configuredEndpoint string, variant string) (*http.Client, error) {
	if p == nil || p.factory == nil {
		return nil, fmt.Errorf("outbound HTTP client pool is not configured")
	}
	endpoint := strings.TrimSpace(configuredEndpoint)
	origin := ""
	policy := p.basePolicy
	if endpoint != "" {
		var err error
		origin, err = security.HTTPOrigin(endpoint)
		if err != nil {
			return nil, fmt.Errorf("validate configured outbound endpoint: %w", err)
		}
		policy, err = p.basePolicy.WithTrustedHTTPURLs(endpoint)
		if err != nil {
			return nil, fmt.Errorf("trust configured outbound endpoint: %w", err)
		}
	}

	key := origin + "\x00" + strings.TrimSpace(variant)
	p.mu.Lock()
	defer p.mu.Unlock()
	if element := p.entries[key]; element != nil {
		p.lru.MoveToFront(element)
		return element.Value.(*cacheEntry).managed.Client, nil
	}
	managed, err := p.factory(policy, origin, strings.TrimSpace(variant))
	if err != nil {
		return nil, err
	}
	if managed.Client == nil {
		return nil, fmt.Errorf("outbound HTTP client factory returned nil client")
	}
	element := p.lru.PushFront(&cacheEntry{key: key, managed: managed})
	p.entries[key] = element
	p.evictLocked()
	return managed.Client, nil
}

// Do 使用与 configuredEndpoint 精确 origin 绑定的复用客户端执行请求。
// configuredEndpoint 为空时不授予局部信任，实际目标继续由严格 transport 校验。
func (p *Pool) Do(request *http.Request, configuredEndpoint string, variant string) (*http.Response, error) {
	if request == nil || request.URL == nil {
		return nil, fmt.Errorf("outbound HTTP request is nil")
	}
	endpoint := strings.TrimSpace(configuredEndpoint)
	if endpoint != "" {
		targetOrigin, err := security.HTTPOrigin(request.URL.String())
		if err != nil {
			return nil, fmt.Errorf("validate outbound request target: %w", err)
		}
		configuredOrigin, err := security.HTTPOrigin(endpoint)
		if err != nil {
			return nil, fmt.Errorf("validate configured outbound endpoint: %w", err)
		}
		if targetOrigin != configuredOrigin {
			return nil, fmt.Errorf("outbound request target changed configured origin")
		}
	}
	client, err := p.clientForEndpoint(endpoint, variant)
	if err != nil {
		return nil, err
	}
	return client.Do(request)
}

// NewRedirectPolicy 保留标准 HTTP 重定向兼容性，同时限制跨 origin 跳转的网络边界。
// 管理员 endpoint 的局部信任只覆盖其精确 origin；跨 origin 目标必须满足 fallbackPolicy，
// 因而公网跳转、显式全局白名单和关闭 SSRF 时的历史行为可以继续工作。
func NewRedirectPolicy(fallbackPolicy security.OutboundPolicy, trustedOrigin string, integration string) func(*http.Request, []*http.Request) error {
	return func(request *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("%s stopped after 10 redirects", integration)
		}
		redirectOrigin, err := security.HTTPOrigin(request.URL.String())
		if err == nil && trustedOrigin != "" && redirectOrigin == trustedOrigin {
			return nil
		}
		if validateErr := security.ValidateOutboundHTTPURL(request.URL.String(), fallbackPolicy); validateErr != nil {
			return fmt.Errorf("%s redirect target is not allowed: %w", integration, validateErr)
		}
		return nil
	}
}

func (p *Pool) evictLocked() {
	for p.lru.Len() > p.limit {
		element := p.lru.Back()
		entry := element.Value.(*cacheEntry)
		delete(p.entries, entry.key)
		p.lru.Remove(element)
		if entry.managed.CloseIdleConnections != nil {
			entry.managed.CloseIdleConnections()
		}
	}
}

// CloseIdleConnections 关闭池中所有传输层的空闲连接。
func (p *Pool) CloseIdleConnections() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for element := p.lru.Front(); element != nil; element = element.Next() {
		entry := element.Value.(*cacheEntry)
		if entry.managed.CloseIdleConnections != nil {
			entry.managed.CloseIdleConnections()
		}
	}
}
