package monitor

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/InfluxCommunity/influxdb3-go/v2/influxdb3"
	"github.com/beego/beego/v2/core/logs"
)

// InfluxConfig holds the subset of app.conf values relevant to the v3 client.
type InfluxConfig struct {
	Enabled       bool
	URL           string
	Token         string
	Database      string
	BufferSize    int
	FlushInterval time.Duration
}

// InfluxWriter buffers points in memory and flushes them to InfluxDB v3
// either every FlushInterval or when the buffer fills. On overflow, the
// oldest points are dropped — InfluxDB is a best-effort side channel;
// SQLite remains the source of truth for history.
type InfluxWriter struct {
	// cfgMu guards cfg + client so Reconfigure can swap them while
	// writers hold only the enqueue/flush mutex.
	cfgMu  sync.RWMutex
	cfg    InfluxConfig
	client *influxdb3.Client

	mu     sync.Mutex
	buffer []*influxdb3.Point

	flushCh chan struct{}
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Rolling 24h counters — bucketed per hour so we can slide a window
	// without a cron. bucketIdx = hour-of-epoch % 24.
	statsMu      sync.Mutex
	flushedBkts  [24]int64
	errorsBkts   [24]int64
	statsHourKey int64 // last seen hour-of-epoch, resets future buckets
	buffered     atomic.Int64
}

// NewInfluxWriter returns a disabled writer if cfg.Enabled is false. The
// returned value is always safe to call WritePoint/Close on.
func NewInfluxWriter(cfg InfluxConfig) (*InfluxWriter, error) {
	w := &InfluxWriter{cfg: cfg}
	if !cfg.Enabled {
		return w, nil
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 10 * time.Second
	}
	client, err := influxdb3.New(influxdb3.ClientConfig{
		Host:     cfg.URL,
		Token:    cfg.Token,
		Database: cfg.Database,
	})
	if err != nil {
		return nil, err
	}
	w.cfg = cfg
	w.client = client
	w.buffer = make([]*influxdb3.Point, 0, cfg.BufferSize)
	w.flushCh = make(chan struct{}, 1)
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.flushLoop()
	return w, nil
}

// Stats reports the current buffer depth and rolling 24-hour point-flush
// success/failure counts. Safe to call from any goroutine; used by the UI.
func (w *InfluxWriter) Stats() (buffered, flushed24h, errors24h int64) {
	if w == nil {
		return 0, 0, 0
	}
	buffered = w.buffered.Load()
	w.statsMu.Lock()
	w.rotateBucketsLocked(time.Now().Unix() / 3600)
	for i := 0; i < 24; i++ {
		flushed24h += w.flushedBkts[i]
		errors24h += w.errorsBkts[i]
	}
	w.statsMu.Unlock()
	return
}

// Config returns a snapshot of the current writer configuration (without the
// token) for display in the UI.
func (w *InfluxWriter) Config() InfluxConfig {
	if w == nil {
		return InfluxConfig{}
	}
	w.cfgMu.RLock()
	defer w.cfgMu.RUnlock()
	return w.cfg
}

// Reconfigure swaps in new connection parameters at runtime. If the new
// config disables InfluxDB, the existing client is closed and the writer
// becomes a no-op until Reconfigure is called again with Enabled=true.
func (w *InfluxWriter) Reconfigure(newCfg InfluxConfig) error {
	w.cfgMu.Lock()
	defer w.cfgMu.Unlock()

	if w.client != nil {
		_ = w.client.Close()
		w.client = nil
	}

	w.cfg = newCfg
	if !newCfg.Enabled {
		return nil
	}
	if newCfg.BufferSize <= 0 {
		newCfg.BufferSize = 1000
		w.cfg.BufferSize = newCfg.BufferSize
	}
	if newCfg.FlushInterval <= 0 {
		newCfg.FlushInterval = 10 * time.Second
		w.cfg.FlushInterval = newCfg.FlushInterval
	}
	client, err := influxdb3.New(influxdb3.ClientConfig{
		Host:     newCfg.URL,
		Token:    newCfg.Token,
		Database: newCfg.Database,
	})
	if err != nil {
		return err
	}
	w.client = client

	// Only start the flush loop if it isn't already running (the writer
	// might have been created as disabled from the start).
	if w.flushCh == nil {
		w.buffer = make([]*influxdb3.Point, 0, newCfg.BufferSize)
		w.flushCh = make(chan struct{}, 1)
		w.stopCh = make(chan struct{})
		w.wg.Add(1)
		go w.flushLoop()
	}
	return nil
}

func (w *InfluxWriter) enabled() bool {
	w.cfgMu.RLock()
	defer w.cfgMu.RUnlock()
	return w.cfg.Enabled
}

func (w *InfluxWriter) rotateBucketsLocked(hourKey int64) {
	if w.statsHourKey == 0 {
		w.statsHourKey = hourKey
		return
	}
	delta := hourKey - w.statsHourKey
	if delta <= 0 {
		return
	}
	if delta >= 24 {
		w.flushedBkts = [24]int64{}
		w.errorsBkts = [24]int64{}
		w.statsHourKey = hourKey
		return
	}
	for i := int64(1); i <= delta; i++ {
		idx := (w.statsHourKey + i) % 24
		w.flushedBkts[idx] = 0
		w.errorsBkts[idx] = 0
	}
	w.statsHourKey = hourKey
}

func (w *InfluxWriter) bumpFlushed(n int64) {
	if n <= 0 {
		return
	}
	w.statsMu.Lock()
	hourKey := time.Now().Unix() / 3600
	w.rotateBucketsLocked(hourKey)
	w.flushedBkts[hourKey%24] += n
	w.statsMu.Unlock()
}

func (w *InfluxWriter) bumpErrors(n int64) {
	if n <= 0 {
		return
	}
	w.statsMu.Lock()
	hourKey := time.Now().Unix() / 3600
	w.rotateBucketsLocked(hourKey)
	w.errorsBkts[hourKey%24] += n
	w.statsMu.Unlock()
}

// WriteTraffic records a per-sample traffic point (cumulative counters).
func (w *InfluxWriter) WriteTraffic(commonName, virtualIP, realIP string, bytesIn, bytesOut int64, at time.Time) {
	if w == nil || !w.enabled() {
		return
	}
	p := influxdb3.NewPointWithMeasurement("openvpn_traffic").
		SetTag("common_name", commonName).
		SetTag("virtual_ip", virtualIP).
		SetTag("real_ip", realIP).
		SetIntegerField("bytes_in", bytesIn).
		SetIntegerField("bytes_out", bytesOut).
		SetTimestamp(at)
	w.enqueue(p)
}

// WriteSession records a completed session with its final byte counts.
func (w *InfluxWriter) WriteSession(commonName, realIP, virtualIP string, bytesIn, bytesOut, durationS int64, at time.Time) {
	if w == nil || !w.enabled() {
		return
	}
	p := influxdb3.NewPointWithMeasurement("openvpn_session").
		SetTag("common_name", commonName).
		SetTag("status", "closed").
		SetIntegerField("bytes_in", bytesIn).
		SetIntegerField("bytes_out", bytesOut).
		SetIntegerField("duration_s", durationS).
		SetStringField("real_ip", realIP).
		SetStringField("virtual_ip", virtualIP).
		SetTimestamp(at)
	w.enqueue(p)
}

func (w *InfluxWriter) enqueue(p *influxdb3.Point) {
	w.cfgMu.RLock()
	bufSize := w.cfg.BufferSize
	w.cfgMu.RUnlock()
	if bufSize <= 0 {
		bufSize = 1000
	}

	w.mu.Lock()
	if len(w.buffer) >= bufSize {
		// drop oldest to keep the newest — we care about freshness in Grafana.
		w.buffer = append(w.buffer[:0], w.buffer[1:]...)
		logs.Warn("InfluxDB buffer overflow — dropping oldest point")
	} else {
		w.buffered.Add(1)
	}
	w.buffer = append(w.buffer, p)
	full := len(w.buffer) >= bufSize
	w.mu.Unlock()
	if full {
		select {
		case w.flushCh <- struct{}{}:
		default:
		}
	}
}

func (w *InfluxWriter) flushLoop() {
	defer w.wg.Done()
	w.cfgMu.RLock()
	interval := w.cfg.FlushInterval
	w.cfgMu.RUnlock()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stopCh:
			w.flush()
			return
		case <-ticker.C:
			w.flush()
		case <-w.flushCh:
			w.flush()
		}
	}
}

func (w *InfluxWriter) flush() {
	w.cfgMu.RLock()
	bufSize := w.cfg.BufferSize
	client := w.client
	w.cfgMu.RUnlock()
	if bufSize <= 0 {
		bufSize = 1000
	}

	w.mu.Lock()
	if len(w.buffer) == 0 {
		w.mu.Unlock()
		return
	}
	batch := w.buffer
	w.buffer = make([]*influxdb3.Point, 0, bufSize)
	w.mu.Unlock()

	w.buffered.Add(-int64(len(batch)))
	if w.buffered.Load() < 0 {
		w.buffered.Store(0)
	}

	if client == nil {
		// Shouldn't happen while enabled, but don't panic if it does.
		w.bumpErrors(int64(len(batch)))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.WritePoints(ctx, batch); err != nil {
		logs.Warn("InfluxDB write failed (%d points dropped): %v", len(batch), err)
		w.bumpErrors(int64(len(batch)))
		return
	}
	w.bumpFlushed(int64(len(batch)))
}

// Close flushes the remaining buffer and releases the underlying client.
func (w *InfluxWriter) Close() error {
	if w == nil {
		return nil
	}
	w.cfgMu.RLock()
	enabled := w.cfg.Enabled
	client := w.client
	stopCh := w.stopCh
	w.cfgMu.RUnlock()
	if !enabled || stopCh == nil {
		return nil
	}
	close(stopCh)
	w.wg.Wait()
	if client != nil {
		return client.Close()
	}
	return nil
}
