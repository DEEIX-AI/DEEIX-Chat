package middleware

import (
	"net"
	"net/netip"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

var (
	trustedProxyHeadersMu sync.RWMutex
	trustedProxyPrefixes  []netip.Prefix
)

// ConfigureTrustedProxyHeaders 配置可被信任的上游代理地址范围。
func ConfigureTrustedProxyHeaders(items []string) error {
	prefixes := make([]netip.Prefix, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}

		if strings.Contains(value, "/") {
			prefix, err := netip.ParsePrefix(value)
			if err != nil {
				return err
			}
			prefixes = append(prefixes, prefix.Masked())
			continue
		}

		addr, err := netip.ParseAddr(value)
		if err != nil {
			return err
		}
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		prefixes = append(prefixes, netip.PrefixFrom(addr, bits))
	}

	trustedProxyHeadersMu.Lock()
	trustedProxyPrefixes = prefixes
	trustedProxyHeadersMu.Unlock()
	return nil
}

func requestCameFromTrustedProxy(c *gin.Context) bool {
	trustedProxyHeadersMu.RLock()
	prefixes := append([]netip.Prefix(nil), trustedProxyPrefixes...)
	trustedProxyHeadersMu.RUnlock()
	if len(prefixes) == 0 || c == nil || c.Request == nil {
		return false
	}

	host := strings.TrimSpace(c.Request.RemoteAddr)
	if host == "" {
		return false
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	for _, prefix := range prefixes {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}
