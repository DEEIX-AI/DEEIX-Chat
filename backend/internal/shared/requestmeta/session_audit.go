package requestmeta

import "strings"

// SessionAuditContext 表示一次认证相关请求的审计上下文。
type SessionAuditContext struct {
	ClientIP     string
	UserAgent    string
	GeoSource    string
	GeoAccuracy  string
	CountryCode  string
	RegionName   string
	CityName     string
	TimezoneName string
	IPLatitude   *float64
	IPLongitude  *float64
}

// Normalize 清理上下文字段中的空白。
func (c SessionAuditContext) Normalize() SessionAuditContext {
	return SessionAuditContext{
		ClientIP:     strings.TrimSpace(c.ClientIP),
		UserAgent:    strings.TrimSpace(c.UserAgent),
		GeoSource:    strings.TrimSpace(c.GeoSource),
		GeoAccuracy:  strings.TrimSpace(c.GeoAccuracy),
		CountryCode:  strings.TrimSpace(c.CountryCode),
		RegionName:   strings.TrimSpace(c.RegionName),
		CityName:     strings.TrimSpace(c.CityName),
		TimezoneName: strings.TrimSpace(c.TimezoneName),
		IPLatitude:   c.IPLatitude,
		IPLongitude:  c.IPLongitude,
	}
}

// LocationLabel 组合位置展示文案。
func (c SessionAuditContext) LocationLabel() string {
	parts := make([]string, 0, 3)
	if city := strings.TrimSpace(c.CityName); city != "" {
		parts = append(parts, city)
	}
	if region := strings.TrimSpace(c.RegionName); region != "" {
		parts = append(parts, region)
	}
	if country := strings.TrimSpace(c.CountryCode); country != "" {
		parts = append(parts, country)
	}
	return strings.Join(parts, ", ")
}
