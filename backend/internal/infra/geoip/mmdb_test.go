package geoip

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMMDBRecordAuditContext(t *testing.T) {
	var record mmdbRecord
	record.RegisteredCountry.ISOCode = "US"
	record.Subdivisions = append(record.Subdivisions, struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	}{
		ISOCode: "CA",
		Names: map[string]string{
			"en": "California",
		},
	})
	record.City.Names = map[string]string{
		"zh-CN": "旧金山",
		"en":    "San Francisco",
	}
	record.Location.TimeZone = "America/Los_Angeles"
	record.Location.Latitude = 37.7749
	record.Location.Longitude = -122.4194

	auditCtx := record.auditContext()
	if auditCtx.GeoSource != "geoip_mmdb" {
		t.Fatalf("unexpected geo source: %s", auditCtx.GeoSource)
	}
	if auditCtx.CountryCode != "US" {
		t.Fatalf("unexpected country code: %s", auditCtx.CountryCode)
	}
	if auditCtx.RegionName != "California" {
		t.Fatalf("unexpected region name: %s", auditCtx.RegionName)
	}
	if auditCtx.CityName != "旧金山" {
		t.Fatalf("unexpected city name: %s", auditCtx.CityName)
	}
	if auditCtx.TimezoneName != "America/Los_Angeles" {
		t.Fatalf("unexpected timezone: %s", auditCtx.TimezoneName)
	}
	if auditCtx.IPLatitude == nil || auditCtx.IPLongitude == nil {
		t.Fatal("expected coordinates")
	}
}

func TestLocalizedNamePriority(t *testing.T) {
	name := localizedName(map[string]string{
		"en":    "Beijing",
		"zh":    "北京",
		"zh-CN": "北京市",
	})
	if name != "北京市" {
		t.Fatalf("unexpected localized name: %s", name)
	}
}

func TestCoordinatesIgnoresEmptyLocation(t *testing.T) {
	lat, lon := coordinates(0, 0)
	if lat != nil || lon != nil {
		t.Fatal("expected empty coordinates")
	}
}

func TestNextRefreshDelayUsesDatabaseMTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "geoip.mmdb")
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	modTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatal(err)
	}

	resolver := &mmdbResolver{
		databasePath:    path,
		refreshInterval: time.Hour,
	}
	if delay := resolver.nextRefreshDelay(); delay != 0 {
		t.Fatalf("expected stale database to refresh immediately, got %s", delay)
	}
}

func TestRetryDelayBackoff(t *testing.T) {
	resolver := &mmdbResolver{refreshInterval: 24 * time.Hour}
	if delay := resolver.retryDelay(1); delay != time.Minute {
		t.Fatalf("unexpected first retry delay: %s", delay)
	}
	if delay := resolver.retryDelay(3); delay != 4*time.Minute {
		t.Fatalf("unexpected third retry delay: %s", delay)
	}
	if delay := resolver.retryDelay(12); delay != time.Hour {
		t.Fatalf("unexpected capped retry delay: %s", delay)
	}
}

func TestMMDBResolverMissingDatabaseIsNonFatal(t *testing.T) {
	resolver := newMMDBResolver(mmdbConfig{
		databasePath:    filepath.Join(t.TempDir(), "missing.mmdb"),
		refreshInterval: time.Hour,
	})
	defer resolver.Close()

	_, err := resolver.Lookup(context.Background(), "8.8.8.8")
	if !errors.Is(err, errMMDBUnavailable) {
		t.Fatalf("expected unavailable database error, got %v", err)
	}
}
