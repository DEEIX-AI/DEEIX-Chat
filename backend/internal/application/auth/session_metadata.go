package auth

import (
	"strings"
	"time"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
)

const sessionActivityTouchInterval = time.Minute

type sessionAuditSnapshot struct {
	ClientIP     string
	UserAgent    string
	DeviceName   string
	BrowserName  string
	OSName       string
	DeviceType   string
	GeoSource    string
	GeoAccuracy  string
	CountryCode  string
	RegionName   string
	CityName     string
	TimezoneName string
	IPLatitude   *float64
	IPLongitude  *float64
}

func buildSessionAuditSnapshot(auditCtx requestmeta.SessionAuditContext) sessionAuditSnapshot {
	normalized := auditCtx.Normalize()
	snapshot := sessionAuditSnapshot{
		ClientIP:     normalized.ClientIP,
		UserAgent:    normalized.UserAgent,
		GeoSource:    normalized.GeoSource,
		GeoAccuracy:  normalized.GeoAccuracy,
		CountryCode:  normalized.CountryCode,
		RegionName:   normalized.RegionName,
		CityName:     normalized.CityName,
		TimezoneName: normalized.TimezoneName,
		IPLatitude:   normalized.IPLatitude,
		IPLongitude:  normalized.IPLongitude,
		BrowserName:  resolveBrowserName(normalized.UserAgent),
		OSName:       resolveOSName(normalized.UserAgent),
		DeviceType:   resolveDeviceType(normalized.UserAgent),
	}
	snapshot.DeviceName = resolveDeviceName(snapshot.DeviceType, snapshot.BrowserName, snapshot.OSName)
	return snapshot
}

func resolveBrowserName(userAgent string) string {
	switch ua := strings.TrimSpace(userAgent); {
	case ua == "":
		return ""
	case strings.Contains(ua, "Edg/"):
		return "Edge"
	case strings.Contains(ua, "OPR/"), strings.Contains(ua, "Opera/"):
		return "Opera"
	case strings.Contains(ua, "Chrome/"), strings.Contains(ua, "CriOS/"):
		return "Chrome"
	case strings.Contains(ua, "Firefox/"), strings.Contains(ua, "FxiOS/"):
		return "Firefox"
	case strings.Contains(ua, "Safari/"):
		return "Safari"
	case strings.Contains(ua, "PostmanRuntime/"):
		return "Postman"
	case strings.Contains(ua, "curl/"):
		return "curl"
	default:
		return ""
	}
}

func resolveOSName(userAgent string) string {
	switch ua := strings.TrimSpace(userAgent); {
	case ua == "":
		return ""
	case strings.Contains(ua, "Windows NT"):
		return "Windows"
	case strings.Contains(ua, "Mac OS X"), strings.Contains(ua, "Macintosh"):
		return "macOS"
	case strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPad"), strings.Contains(ua, "iPod"):
		return "iOS"
	case strings.Contains(ua, "Android"):
		return "Android"
	case strings.Contains(ua, "Linux"):
		return "Linux"
	default:
		return ""
	}
}

func resolveDeviceType(userAgent string) string {
	switch ua := strings.TrimSpace(userAgent); {
	case ua == "":
		return "unknown"
	case strings.Contains(ua, "bot"), strings.Contains(ua, "spider"), strings.Contains(ua, "crawler"):
		return "bot"
	case strings.Contains(ua, "iPad"), strings.Contains(ua, "Tablet"):
		return "tablet"
	case strings.Contains(ua, "Mobile"), strings.Contains(ua, "Android"), strings.Contains(ua, "iPhone"), strings.Contains(ua, "iPod"):
		return "mobile"
	default:
		return "desktop"
	}
}

func resolveDeviceName(deviceType string, browserName string, osName string) string {
	if browserName != "" && osName != "" {
		return browserName + " on " + osName
	}
	if browserName != "" {
		return browserName
	}
	if osName != "" {
		return osName
	}

	switch strings.TrimSpace(deviceType) {
	case "mobile":
		return "Mobile device"
	case "tablet":
		return "Tablet"
	case "bot":
		return "Automation client"
	case "desktop":
		return "Desktop browser"
	default:
		return "Unknown device"
	}
}

func shouldTouchSessionActivity(session *domainuser.Session, snapshot sessionAuditSnapshot, now time.Time) bool {
	if session == nil {
		return false
	}
	if session.LastSeenAt == nil || now.Sub(*session.LastSeenAt) >= sessionActivityTouchInterval {
		return true
	}

	return strings.TrimSpace(session.ClientIP) != snapshot.ClientIP ||
		strings.TrimSpace(session.UserAgent) != snapshot.UserAgent ||
		strings.TrimSpace(session.DeviceName) != snapshot.DeviceName ||
		strings.TrimSpace(session.BrowserName) != snapshot.BrowserName ||
		strings.TrimSpace(session.OSName) != snapshot.OSName ||
		strings.TrimSpace(session.DeviceType) != snapshot.DeviceType ||
		strings.TrimSpace(session.GeoSource) != snapshot.GeoSource ||
		strings.TrimSpace(session.GeoAccuracy) != snapshot.GeoAccuracy ||
		strings.TrimSpace(session.CountryCode) != snapshot.CountryCode ||
		strings.TrimSpace(session.RegionName) != snapshot.RegionName ||
		strings.TrimSpace(session.CityName) != snapshot.CityName ||
		strings.TrimSpace(session.TimezoneName) != snapshot.TimezoneName ||
		optionalFloatChanged(session.IPLatitude, snapshot.IPLatitude) ||
		optionalFloatChanged(session.IPLongitude, snapshot.IPLongitude)
}

func resolveSessionLocationLabel(session *domainuser.Session) string {
	if session == nil {
		return ""
	}
	return requestmeta.SessionAuditContext{
		CountryCode: session.CountryCode,
		RegionName:  session.RegionName,
		CityName:    session.CityName,
	}.Normalize().LocationLabel()
}

func optionalFloatChanged(current *float64, next *float64) bool {
	if current == nil && next == nil {
		return false
	}
	if current == nil || next == nil {
		return true
	}
	return *current != *next
}

func resolveSessionDeviceLabel(session *domainuser.Session) string {
	if session == nil {
		return ""
	}
	if value := strings.TrimSpace(session.DeviceName); value != "" {
		return value
	}
	return resolveDeviceName(
		strings.TrimSpace(session.DeviceType),
		strings.TrimSpace(session.BrowserName),
		strings.TrimSpace(session.OSName),
	)
}

func marshalSessionAuthEventDetail(sessionID string, snapshot sessionAuditSnapshot) string {
	return marshalAuthEventDetail(map[string]interface{}{
		"session_id":   strings.TrimSpace(sessionID),
		"device_name":  snapshot.DeviceName,
		"browser_name": snapshot.BrowserName,
		"os_name":      snapshot.OSName,
		"device_type":  snapshot.DeviceType,
		"geo_source":   snapshot.GeoSource,
		"geo_accuracy": snapshot.GeoAccuracy,
		"client_ip":    snapshot.ClientIP,
		"country_code": snapshot.CountryCode,
		"region_name":  snapshot.RegionName,
		"city_name":    snapshot.CityName,
		"timezone":     snapshot.TimezoneName,
		"ip_latitude":  snapshot.IPLatitude,
		"ip_longitude": snapshot.IPLongitude,
		"location_label": requestmeta.SessionAuditContext{
			CountryCode: snapshot.CountryCode,
			RegionName:  snapshot.RegionName,
			CityName:    snapshot.CityName,
		}.Normalize().LocationLabel(),
	})
}
