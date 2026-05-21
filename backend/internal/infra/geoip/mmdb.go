package geoip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/requestmeta"
	"github.com/oschwald/maxminddb-golang"
)

var errMMDBUnavailable = errors.New("geoip mmdb database unavailable")

type mmdbConfig struct {
	databaseURL      string
	databasePath     string
	databaseMaxBytes int64
	refreshInterval  time.Duration
	httpClient       *http.Client
}

type mmdbResolver struct {
	databaseURL      string
	databasePath     string
	databaseMaxBytes int64
	refreshInterval  time.Duration
	httpClient       *http.Client

	mu        sync.RWMutex
	refreshMu sync.Mutex
	reader    *maxminddb.Reader

	started  bool
	stopOnce sync.Once
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func newMMDBResolver(cfg mmdbConfig) *mmdbResolver {
	databasePath := strings.TrimSpace(cfg.databasePath)
	if databasePath == "" {
		databasePath = "./data/geoip/geoip.mmdb"
	}
	httpClient := cfg.httpClient
	if httpClient == nil {
		httpClient = platformtracing.NewHTTPClient(2500 * time.Millisecond)
	}

	resolver := &mmdbResolver{
		databaseURL:      strings.TrimSpace(cfg.databaseURL),
		databasePath:     databasePath,
		databaseMaxBytes: cfg.databaseMaxBytes,
		refreshInterval:  cfg.refreshInterval,
		httpClient:       httpClient,
		stopCh:           make(chan struct{}),
		doneCh:           make(chan struct{}),
	}
	_ = resolver.loadInitial(context.Background())
	resolver.startRefreshLoop()
	return resolver
}

func (r *mmdbResolver) Lookup(ctx context.Context, ip string) (requestmeta.SessionAuditContext, error) {
	select {
	case <-ctx.Done():
		return requestmeta.SessionAuditContext{}, ctx.Err()
	default:
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return requestmeta.SessionAuditContext{}, errLookupSkipped
	}

	var record mmdbRecord
	r.mu.RLock()
	reader := r.reader
	if reader == nil {
		r.mu.RUnlock()
		return requestmeta.SessionAuditContext{}, errMMDBUnavailable
	}
	err := reader.Lookup(parsed, &record)
	r.mu.RUnlock()
	if err != nil {
		return requestmeta.SessionAuditContext{}, err
	}

	result := record.auditContext()
	if result.CountryCode == "" && result.RegionName == "" && result.CityName == "" && result.TimezoneName == "" {
		return requestmeta.SessionAuditContext{}, errLookupSkipped
	}
	return result, nil
}

func (r *mmdbResolver) Close() {
	if r == nil {
		return
	}
	if r.started {
		r.stopOnce.Do(func() {
			close(r.stopCh)
			<-r.doneCh
		})
	}

	r.mu.Lock()
	reader := r.reader
	r.reader = nil
	r.mu.Unlock()
	if reader != nil {
		_ = reader.Close()
	}
}

func (r *mmdbResolver) loadInitial(ctx context.Context) error {
	if err := r.loadDatabase(); err == nil {
		return nil
	}
	if r.databaseURL == "" {
		return errMMDBUnavailable
	}
	return r.refreshDatabase(ctx)
}

func (r *mmdbResolver) startRefreshLoop() {
	if r.databaseURL == "" || r.refreshInterval <= 0 {
		return
	}
	r.started = true
	go func() {
		defer close(r.doneCh)

		timer := time.NewTimer(r.nextRefreshDelay())
		defer timer.Stop()

		failures := 0
		for {
			select {
			case <-timer.C:
				nextDelay := r.nextRefreshDelay()
				if err := r.refreshDatabase(context.Background()); err != nil {
					failures++
					nextDelay = r.retryDelay(failures)
				} else {
					failures = 0
				}
				timer.Reset(nextDelay)
			case <-r.stopCh:
				return
			}
		}
	}()
}

func (r *mmdbResolver) refreshDatabase(ctx context.Context) error {
	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()

	if r.databaseURL == "" {
		return errMMDBUnavailable
	}
	if err := os.MkdirAll(filepath.Dir(r.databasePath), 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(r.databasePath), ".geoip-*.mmdb")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if err = r.downloadDatabase(ctx, tmp); err != nil {
		_ = tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	reader, err := openMMDBReader(tmpName)
	if err != nil {
		return err
	}
	if err = os.Rename(tmpName, r.databasePath); err != nil {
		_ = reader.Close()
		return err
	}
	r.replaceReader(reader)
	return nil
}

func (r *mmdbResolver) downloadDatabase(ctx context.Context, target io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.databaseURL, nil)
	if err != nil {
		return err
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("geoip mmdb download failed: %s", resp.Status)
	}
	if r.databaseMaxBytes > 0 && resp.ContentLength > r.databaseMaxBytes {
		return fmt.Errorf("geoip mmdb download exceeds max size: %d > %d", resp.ContentLength, r.databaseMaxBytes)
	}

	reader := io.Reader(resp.Body)
	if r.databaseMaxBytes > 0 {
		reader = io.LimitReader(resp.Body, r.databaseMaxBytes+1)
	}
	written, err := io.Copy(target, reader)
	if err != nil {
		return err
	}
	if r.databaseMaxBytes > 0 && written > r.databaseMaxBytes {
		return fmt.Errorf("geoip mmdb download exceeds max size: %d > %d", written, r.databaseMaxBytes)
	}
	return nil
}

func (r *mmdbResolver) loadDatabase() error {
	reader, err := openMMDBReader(r.databasePath)
	if err != nil {
		return err
	}
	r.replaceReader(reader)
	return nil
}

func openMMDBReader(path string) (*maxminddb.Reader, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader, err := maxminddb.FromBytes(data)
	if err != nil {
		return nil, err
	}
	return reader, nil
}

func (r *mmdbResolver) replaceReader(reader *maxminddb.Reader) {
	r.mu.Lock()
	oldReader := r.reader
	r.reader = reader
	r.mu.Unlock()
	if oldReader != nil {
		_ = oldReader.Close()
	}
}

func (r *mmdbResolver) nextRefreshDelay() time.Duration {
	info, err := os.Stat(r.databasePath)
	if err != nil {
		return 0
	}
	delay := time.Until(info.ModTime().Add(r.refreshInterval))
	if delay < 0 {
		return 0
	}
	return delay
}

func (r *mmdbResolver) retryDelay(failures int) time.Duration {
	if failures <= 0 {
		failures = 1
	}
	delay := time.Minute
	if r.refreshInterval < delay {
		delay = r.refreshInterval
	}
	for i := 1; i < failures; i++ {
		delay *= 2
	}
	capDelay := time.Hour
	if r.refreshInterval < capDelay {
		capDelay = r.refreshInterval
	}
	if delay > capDelay {
		return capDelay
	}
	return delay
}

type mmdbRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	RegisteredCountry struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"registered_country"`
	Subdivisions []struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
		TimeZone  string  `maxminddb:"time_zone"`
	} `maxminddb:"location"`
}

func (r mmdbRecord) auditContext() requestmeta.SessionAuditContext {
	lat, lon := coordinates(r.Location.Latitude, r.Location.Longitude)
	return requestmeta.SessionAuditContext{
		GeoSource:    "geoip_mmdb",
		GeoAccuracy:  "ip",
		CountryCode:  firstNonEmpty(r.Country.ISOCode, r.RegisteredCountry.ISOCode),
		RegionName:   firstSubdivisionName(r.Subdivisions),
		CityName:     localizedName(r.City.Names),
		TimezoneName: r.Location.TimeZone,
		IPLatitude:   lat,
		IPLongitude:  lon,
	}.Normalize()
}

func firstSubdivisionName(subdivisions []struct {
	ISOCode string            `maxminddb:"iso_code"`
	Names   map[string]string `maxminddb:"names"`
}) string {
	if len(subdivisions) == 0 {
		return ""
	}
	return firstNonEmpty(localizedName(subdivisions[0].Names), subdivisions[0].ISOCode)
}

func localizedName(names map[string]string) string {
	for _, locale := range []string{"zh-CN", "zh", "en"} {
		if name := strings.TrimSpace(names[locale]); name != "" {
			return name
		}
	}
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func coordinates(lat float64, lon float64) (*float64, *float64) {
	if lat == 0 && lon == 0 {
		return nil, nil
	}
	return &lat, &lon
}
