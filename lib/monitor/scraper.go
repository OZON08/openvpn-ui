package monitor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/server/web"

	"github.com/OZON08/openvpn-ui/models"
)

// Config holds the scraper and retention settings read from app.conf.
type Config struct {
	Enabled               bool
	ScrapeInterval        time.Duration
	SampleRetentionDays   int
	HourlyRetentionDays   int
	HookToken             string
	StatusLogPath         string
	Influx                InfluxConfig
}

// LoadConfig reads monitoring settings from Beego's web.AppConfig and returns
// a fully-populated Config. Missing values fall back to sane defaults.
func LoadConfig() Config {
	cfg := Config{
		Enabled:             web.AppConfig.DefaultBool("MonitoringEnabled", true),
		ScrapeInterval:      time.Duration(web.AppConfig.DefaultInt("MonitoringScrapeIntervalS", 60)) * time.Second,
		SampleRetentionDays: web.AppConfig.DefaultInt("MonitoringSampleRetentionDays", 30),
		HourlyRetentionDays: web.AppConfig.DefaultInt("MonitoringHourlyRetentionDays", 365),
		HookToken:           web.AppConfig.DefaultString("MonitoringHookToken", ""),
	}

	ovConfigPath := web.AppConfig.DefaultString("OpenVpnPath", "./openvpn")
	// The OpenVPN server typically writes status to <ovpath>/log/openvpn-status.log.
	cfg.StatusLogPath = filepath.Join(ovConfigPath, "log", "openvpn-status.log")
	if p := web.AppConfig.DefaultString("MonitoringStatusLogPath", ""); p != "" {
		cfg.StatusLogPath = p
	}

	cfg.Influx = InfluxConfig{
		Enabled:       web.AppConfig.DefaultBool("InfluxEnabled", false),
		URL:           web.AppConfig.DefaultString("InfluxURL", ""),
		Token:         web.AppConfig.DefaultString("InfluxToken", ""),
		Database:      web.AppConfig.DefaultString("InfluxDatabase", "openvpn"),
		BufferSize:    web.AppConfig.DefaultInt("InfluxBufferSize", 1000),
		FlushInterval: time.Duration(web.AppConfig.DefaultInt("InfluxFlushIntervalS", 10)) * time.Second,
	}
	return cfg
}

// Scraper is the top-level monitor runtime. It owns the status.log poller,
// the InfluxDB writer, and the retention GC.
type Scraper struct {
	cfg    Config
	influx *InfluxWriter

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Start boots the monitor background goroutines. It is safe to call when
// MonitoringEnabled is false — Start returns nil and does nothing.
func Start() (*Scraper, error) {
	cfg := LoadConfig()
	if !cfg.Enabled {
		logs.Info("Monitoring disabled via MonitoringEnabled=false")
		return nil, nil
	}

	influx, err := NewInfluxWriter(cfg.Influx)
	if err != nil {
		logs.Warn("InfluxDB init failed, continuing without it: %v", err)
		influx = &InfluxWriter{cfg: InfluxConfig{Enabled: false}}
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Scraper{cfg: cfg, influx: influx, cancel: cancel}

	s.wg.Add(2)
	go s.scrapeLoop(ctx)
	go s.retentionLoop(ctx)

	logs.Info("Monitoring started (interval=%s, status=%s, influx=%v)",
		cfg.ScrapeInterval, cfg.StatusLogPath, cfg.Influx.Enabled)
	return s, nil
}

// Stop flushes pending data and tears down the background goroutines.
func (s *Scraper) Stop() {
	if s == nil {
		return
	}
	s.cancel()
	s.wg.Wait()
	if s.influx != nil {
		_ = s.influx.Close()
	}
}

// Influx exposes the writer so the HTTP disconnect-hook can reuse it.
func (s *Scraper) Influx() *InfluxWriter { return s.influx }

// HookToken returns the shared secret for the disconnect webhook.
func (s *Scraper) HookToken() string { return s.cfg.HookToken }

// scrapeLoop polls openvpn-status.log and updates SQLite + InfluxDB.
func (s *Scraper) scrapeLoop(ctx context.Context) {
	defer s.wg.Done()
	// First tick immediately so dashboards light up on startup.
	s.scrapeOnce()
	ticker := time.NewTicker(s.cfg.ScrapeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scrapeOnce()
		}
	}
}

func (s *Scraper) scrapeOnce() {
	if _, err := os.Stat(s.cfg.StatusLogPath); err != nil {
		// Not fatal — the server may not be running yet.
		logs.Debug("status.log not available: %v", err)
		return
	}

	clients, err := ParseStatusLog(s.cfg.StatusLogPath)
	if err != nil {
		logs.Warn("status.log parse failed: %v", err)
		return
	}

	now := time.Now().UTC()
	seen := make(map[int64]struct{}, len(clients))

	for _, c := range clients {
		session, err := s.upsertSession(c, now)
		if err != nil {
			logs.Warn("upsert session for %s failed: %v", c.CommonName, err)
			continue
		}
		seen[session.Id] = struct{}{}

		sample := &models.TrafficSample{
			SessionId:  session.Id,
			CommonName: c.CommonName,
			VirtualIP:  c.VirtualIP,
			RealIP:     c.RealIP,
			BytesIn:    c.BytesReceived,
			BytesOut:   c.BytesSent,
			SampledAt:  now,
		}
		if err := sample.Insert(); err != nil {
			logs.Warn("traffic sample insert failed: %v", err)
		}

		// Keep VpnSession counters mirroring the latest cumulative values.
		session.BytesIn = c.BytesReceived
		session.BytesOut = c.BytesSent
		if err := session.Update("BytesIn", "BytesOut"); err != nil {
			logs.Warn("session counter update failed: %v", err)
		}

		s.influx.WriteTraffic(c.CommonName, c.VirtualIP, c.RealIP, c.BytesReceived, c.BytesSent, now)
	}

	// Close any active sessions that did not show up in this scrape — the
	// disconnect-hook will normally beat us to it, but this is the safety net.
	s.closeMissingSessions(seen, now)
}

// upsertSession returns the active VpnSession for a client, creating a new
// one if the reported ConnectedAt timestamp does not match any active row.
func (s *Scraper) upsertSession(c StatusClient, now time.Time) (*models.VpnSession, error) {
	if c.CommonName == "" {
		return nil, errors.New("empty common name")
	}
	if !c.ConnectedAt.IsZero() {
		if existing, err := models.FindActiveSession(c.CommonName, c.ConnectedAt); err == nil && existing != nil {
			return existing, nil
		}
	}
	// No matching active session — close any stale actives for this CN first,
	// then open a fresh one. This handles reconnects cleanly.
	if stale, err := models.FindActiveSessionsByCN(c.CommonName); err == nil {
		for _, st := range stale {
			st.Status = "closed"
			st.DisconnectedAt = now
			st.DurationS = int64(now.Sub(st.ConnectedAt).Seconds())
			_ = st.Update("Status", "DisconnectedAt", "DurationS")
		}
	}

	session := &models.VpnSession{
		CommonName:  c.CommonName,
		RealIP:      c.RealIP,
		VirtualIP:   c.VirtualIP,
		ConnectedAt: pickConnectedAt(c.ConnectedAt, now),
		BytesIn:     c.BytesReceived,
		BytesOut:    c.BytesSent,
		Status:      "active",
	}
	if err := session.Insert(); err != nil {
		return nil, err
	}
	return session, nil
}

// closeMissingSessions marks every active session not in `seen` as closed.
func (s *Scraper) closeMissingSessions(seen map[int64]struct{}, now time.Time) {
	active, err := models.ListActiveSessions()
	if err != nil {
		return
	}
	for _, st := range active {
		if _, ok := seen[st.Id]; ok {
			continue
		}
		st.Status = "closed"
		st.DisconnectedAt = now
		st.DurationS = int64(now.Sub(st.ConnectedAt).Seconds())
		if err := st.Update("Status", "DisconnectedAt", "DurationS"); err != nil {
			logs.Warn("closing stale session %d failed: %v", st.Id, err)
			continue
		}
		s.influx.WriteSession(st.CommonName, st.RealIP, st.VirtualIP, st.BytesIn, st.BytesOut, st.DurationS, now)
	}
}

func pickConnectedAt(reported, now time.Time) time.Time {
	if reported.IsZero() {
		return now
	}
	return reported
}

// retentionLoop aggregates old samples into TrafficHourly / TrafficDaily and
// prunes stale rows. Runs once at startup (after a short delay) and hourly.
func (s *Scraper) retentionLoop(ctx context.Context) {
	defer s.wg.Done()
	// Stagger the first run so it doesn't fight with startup work.
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Minute):
	}
	s.runRetention()
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runRetention()
		}
	}
}

func (s *Scraper) runRetention() {
	if err := AggregateSamplesToHourly(s.cfg.SampleRetentionDays); err != nil {
		logs.Warn("hourly aggregation failed: %v", err)
	}
	if err := AggregateHourlyToDaily(s.cfg.HourlyRetentionDays); err != nil {
		logs.Warn("daily aggregation failed: %v", err)
	}
	if err := PruneOldSamples(s.cfg.SampleRetentionDays); err != nil {
		logs.Warn("sample prune failed: %v", err)
	}
	if err := PruneOldHourly(s.cfg.HourlyRetentionDays); err != nil {
		logs.Warn("hourly prune failed: %v", err)
	}
	// Reclaim space on SQLite.
	if _, err := orm.NewOrm().Raw("VACUUM").Exec(); err != nil {
		logs.Debug("VACUUM skipped: %v", err)
	}
}
