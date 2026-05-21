package middleware

import (
	"strconv"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
	"github.com/gin-gonic/gin"
)

var countryHeaderKeys = []string{
	"CF-IPCountry",
	"X-Vercel-IP-Country",
	"X-Appengine-Country",
	"X-Geo-Country",
	"X-Country-Code",
}

var regionHeaderKeys = []string{
	"X-Vercel-IP-Country-Region",
	"X-Appengine-Region",
	"X-Geo-Region",
	"X-Region",
}

var cityHeaderKeys = []string{
	"X-Vercel-IP-City",
	"X-Appengine-City",
	"X-Geo-City",
	"X-City",
}

var timezoneHeaderKeys = []string{
	"CF-Timezone",
	"X-Vercel-IP-Timezone",
	"X-Timezone",
}

var latitudeHeaderKeys = []string{
	"CF-IPLatitude",
	"X-Vercel-IP-Latitude",
	"X-Geo-Latitude",
	"X-Latitude",
}

var longitudeHeaderKeys = []string{
	"CF-IPLongitude",
	"X-Vercel-IP-Longitude",
	"X-Geo-Longitude",
	"X-Longitude",
}

// ResolveSessionAuditContext 从请求中提取会话审计上下文。
func ResolveSessionAuditContext(c *gin.Context) requestmeta.SessionAuditContext {
	countryCode := ""
	regionName := ""
	cityName := ""
	timezoneName := ""
	var ipLatitude *float64
	var ipLongitude *float64
	geoSource := ""
	geoAccuracy := ""

	if requestCameFromTrustedProxy(c) {
		countryCode = firstHeaderValue(c, countryHeaderKeys)
		regionName = firstHeaderValue(c, regionHeaderKeys)
		cityName = firstHeaderValue(c, cityHeaderKeys)
		timezoneName = firstHeaderValue(c, timezoneHeaderKeys)
		ipLatitude = parseOptionalFloat(firstHeaderValue(c, latitudeHeaderKeys))
		ipLongitude = parseOptionalFloat(firstHeaderValue(c, longitudeHeaderKeys))
		if countryCode != "" || regionName != "" || cityName != "" || timezoneName != "" || ipLatitude != nil || ipLongitude != nil {
			geoSource = "proxy_header_trusted"
			geoAccuracy = "ip"
		}
	}

	return requestmeta.SessionAuditContext{
		ClientIP:     c.ClientIP(),
		UserAgent:    c.Request.UserAgent(),
		GeoSource:    geoSource,
		GeoAccuracy:  geoAccuracy,
		CountryCode:  countryCode,
		RegionName:   regionName,
		CityName:     cityName,
		TimezoneName: timezoneName,
		IPLatitude:   ipLatitude,
		IPLongitude:  ipLongitude,
	}.Normalize()
}

func firstHeaderValue(c *gin.Context, keys []string) string {
	for _, key := range keys {
		value := strings.TrimSpace(c.GetHeader(key))
		if value != "" {
			return value
		}
	}
	return ""
}

func parseOptionalFloat(raw string) *float64 {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}
	return &parsed
}
