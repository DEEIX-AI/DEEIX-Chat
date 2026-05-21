package geoip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

var errLookupSkipped = errors.New("geo lookup skipped")

// Client 提供粗略 IP 地理位置解析能力。
type Client struct {
	provider   string
	baseURL    string
	token      string
	httpClient *http.Client
	mmdb       *mmdbResolver
}

// New 创建 GeoIP 客户端。
func New(cfg config.Config) *Client {
	provider := strings.TrimSpace(strings.ToLower(cfg.GeoIPProvider))
	if provider == "" || provider == "none" || provider == "disabled" {
		return nil
	}

	timeout := time.Duration(cfg.GeoIPTimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 2500 * time.Millisecond
	}
	transport := security.NewOutboundHTTPTransport(cfg.Env, 10*time.Second)
	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: platformtracing.NewHTTPTransport(transport),
	}

	if provider == "mmdb" {
		return &Client{
			provider:   provider,
			httpClient: httpClient,
			mmdb: newMMDBResolver(mmdbConfig{
				databaseURL:      cfg.GeoIPDatabaseURL,
				databasePath:     cfg.GeoIPDatabasePath,
				databaseMaxBytes: cfg.GeoIPDatabaseMaxBytes,
				refreshInterval:  time.Duration(cfg.GeoIPRefreshIntervalHours) * time.Hour,
				httpClient:       httpClient,
			}),
		}
	}

	baseURL := strings.TrimSpace(cfg.GeoIPBaseURL)
	if baseURL == "" {
		switch provider {
		case "ipinfo":
			baseURL = "https://ipinfo.io"
		default:
			baseURL = "https://ipwho.is"
		}
	}

	return &Client{
		provider:   provider,
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      strings.TrimSpace(cfg.GeoIPToken),
		httpClient: httpClient,
	}
}

// Close 释放 GeoIP 客户端持有的本地数据库资源。
func (c *Client) Close() {
	if c == nil || c.mmdb == nil {
		return
	}
	c.mmdb.Close()
}

// Lookup 根据公网 IP 解析粗略位置。
func (c *Client) Lookup(ctx context.Context, rawIP string) (requestmeta.SessionAuditContext, error) {
	if c == nil {
		return requestmeta.SessionAuditContext{}, errLookupSkipped
	}

	ip, err := normalizeLookupIP(rawIP)
	if err != nil {
		return requestmeta.SessionAuditContext{}, err
	}

	switch c.provider {
	case "mmdb":
		if c.mmdb == nil {
			return requestmeta.SessionAuditContext{}, errLookupSkipped
		}
		return c.mmdb.Lookup(ctx, ip)
	case "ipinfo":
		return c.lookupIPInfo(ctx, ip)
	default:
		return c.lookupIPWhois(ctx, ip)
	}
}

func normalizeLookupIP(rawIP string) (string, error) {
	value := strings.TrimSpace(rawIP)
	if value == "" {
		return "", errLookupSkipped
	}

	if parsed := net.ParseIP(value); parsed != nil {
		addr, ok := netip.AddrFromSlice(parsed)
		if !ok {
			return "", errLookupSkipped
		}
		return validateLookupAddr(addr)
	}

	host, _, err := net.SplitHostPort(value)
	if err == nil {
		addr, parseErr := netip.ParseAddr(host)
		if parseErr != nil {
			return "", parseErr
		}
		return validateLookupAddr(addr)
	}

	addr, err := netip.ParseAddr(value)
	if err != nil {
		return "", err
	}
	return validateLookupAddr(addr)
}

func validateLookupAddr(addr netip.Addr) (string, error) {
	if !addr.IsValid() || !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		return "", errLookupSkipped
	}
	return addr.String(), nil
}

type ipWhoisResponse struct {
	Success     bool     `json:"success"`
	CountryCode string   `json:"country_code"`
	Region      string   `json:"region"`
	City        string   `json:"city"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	Timezone    struct {
		ID string `json:"id"`
	} `json:"timezone"`
	Message string `json:"message"`
}

func (c *Client) lookupIPWhois(ctx context.Context, ip string) (requestmeta.SessionAuditContext, error) {
	endpoint := c.baseURL + "/" + url.PathEscape(ip)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return requestmeta.SessionAuditContext{}, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return requestmeta.SessionAuditContext{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return requestmeta.SessionAuditContext{}, fmt.Errorf("geo lookup failed: %s", resp.Status)
	}

	var payload ipWhoisResponse
	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return requestmeta.SessionAuditContext{}, err
	}
	if !payload.Success {
		if payload.Message == "" {
			payload.Message = "ipwhois lookup unsuccessful"
		}
		return requestmeta.SessionAuditContext{}, errors.New(payload.Message)
	}

	return requestmeta.SessionAuditContext{
		GeoSource:    "geoip_api",
		GeoAccuracy:  "ip",
		CountryCode:  payload.CountryCode,
		RegionName:   payload.Region,
		CityName:     payload.City,
		TimezoneName: payload.Timezone.ID,
		IPLatitude:   payload.Latitude,
		IPLongitude:  payload.Longitude,
	}.Normalize(), nil
}

type ipInfoResponse struct {
	Country  string `json:"country"`
	Region   string `json:"region"`
	City     string `json:"city"`
	Timezone string `json:"timezone"`
	Loc      string `json:"loc"`
}

func (c *Client) lookupIPInfo(ctx context.Context, ip string) (requestmeta.SessionAuditContext, error) {
	endpoint := c.baseURL + "/" + url.PathEscape(ip) + "/json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return requestmeta.SessionAuditContext{}, err
	}
	query := req.URL.Query()
	if c.token != "" {
		query.Set("token", c.token)
		req.URL.RawQuery = query.Encode()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return requestmeta.SessionAuditContext{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return requestmeta.SessionAuditContext{}, fmt.Errorf("geo lookup failed: %s", resp.Status)
	}

	var payload ipInfoResponse
	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return requestmeta.SessionAuditContext{}, err
	}

	lat, lon := parseLoc(payload.Loc)
	return requestmeta.SessionAuditContext{
		GeoSource:    "geoip_api",
		GeoAccuracy:  "ip",
		CountryCode:  payload.Country,
		RegionName:   payload.Region,
		CityName:     payload.City,
		TimezoneName: payload.Timezone,
		IPLatitude:   lat,
		IPLongitude:  lon,
	}.Normalize(), nil
}

func parseLoc(raw string) (*float64, *float64) {
	parts := strings.Split(strings.TrimSpace(raw), ",")
	if len(parts) != 2 {
		return nil, nil
	}
	latValue, err := strconvParseFloat(parts[0])
	if err != nil {
		return nil, nil
	}
	lonValue, err := strconvParseFloat(parts[1])
	if err != nil {
		return nil, nil
	}
	return &latValue, &lonValue
}

func strconvParseFloat(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	return strconv.ParseFloat(value, 64)
}
