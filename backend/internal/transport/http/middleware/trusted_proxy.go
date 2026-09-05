package middleware

import (
	"net"
	"net/netip"
	"strings"

	"github.com/gin-gonic/gin"
)

const trustedProxyHeadersContextKey = "ctx_trusted_proxy_headers"

// TrustedProxyHeaders 创建可信代理头识别中间件。
func TrustedProxyHeaders(items []string) (gin.HandlerFunc, error) {
	prefixes := make([]netip.Prefix, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}

		if strings.Contains(value, "/") {
			prefix, err := netip.ParsePrefix(value)
			if err != nil {
				return nil, err
			}
			prefixes = append(prefixes, prefix.Masked())
			continue
		}

		addr, err := netip.ParseAddr(value)
		if err != nil {
			return nil, err
		}
		bits := 32
		if addr.Is6() {
			bits = 128
		}
		prefixes = append(prefixes, netip.PrefixFrom(addr, bits))
	}

	return func(c *gin.Context) {
		c.Set(trustedProxyHeadersContextKey, remoteAddressMatchesPrefixes(c, prefixes))
		c.Next()
	}, nil
}

func requestCameFromTrustedProxy(c *gin.Context) bool {
	if c == nil {
		return false
	}
	trusted, _ := c.Get(trustedProxyHeadersContextKey)
	return trusted == true
}

func remoteAddressMatchesPrefixes(c *gin.Context, prefixes []netip.Prefix) bool {
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
