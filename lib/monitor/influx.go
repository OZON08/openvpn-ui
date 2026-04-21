package monitor

import (
	"context"
	"sync"
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
	cfg    InfluxConfig
	client *influxdb3.Client

	mu     sync.Mutex
	buffer []*influxdb3.Point

	flushCh chan struct{}
	stopCh  chan struct{}
	wg      sync.WaitGroup
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
	w.client = client
	w.buffer = make([]*influxdb3.Point, 0, cfg.BufferSize)
	w.flushCh = make(chan struct{}, 1)
	w.stopCh = make(chan struct{})

	w.wg.Add(1)
	go w.flushLoop()
	return w, nil
}

// WriteTraffic records a per-sample traffic point (cumulative counters).
func (w *InfluxWriter) WriteTraffic(commonName, virtualIP, realIP string, bytesIn, bytesOut int64, at time.Time) {
	if w == nil || !w.cfg.Enabled {
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
	if w == nil || !w.cfg.Enabled {
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
	w.mu.Lock()
	if len(w.buffer) >= w.cfg.BufferSize {
		// drop oldest to keep the newest — we care about freshness in Grafana.
		w.buffer = append(w.buffer[:0], w.buffer[1:]...)
		logs.Warn("InfluxDB buffer overflow — dropping oldest point")
	}
	w.buffer = append(w.buffer, p)
	full := len(w.buffer) >= w.cfg.BufferSize
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
	ticker := time.NewTicker(w.cfg.FlushInterval)
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
	w.mu.Lock()
	if len(w.buffer) == 0 {
		w.mu.Unlock()
		return
	}
	batch := w.buffer
	w.buffer = make([]*influxdb3.Point, 0, w.cfg.BufferSize)
	w.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := w.client.WritePoints(ctx, batch); err != nil {
		logs.Warn("InfluxDB write failed (%d points dropped): %v", len(batch), err)
	}
}

// Close flushes the remaining buffer and releases the underlying client.
func (w *InfluxWriter) Close() error {
	if w == nil || !w.cfg.Enabled {
		return nil
	}
	close(w.stopCh)
	w.wg.Wait()
	if w.client != nil {
		return w.client.Close()
	}
	return nil
}
