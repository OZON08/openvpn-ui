package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
)

// VpnSession represents a single connect/disconnect cycle of a client.
// It is opened by the scraper when the client first appears in status.log
// (or by a client-connect hook, if wired up), and closed either by the
// client-disconnect hook or by the scraper when the client disappears.
type VpnSession struct {
	Id             int64
	CommonName     string    `orm:"size(128);index"`
	RealIP         string    `orm:"size(45)"`
	VirtualIP      string    `orm:"size(45)"`
	ConnectedAt    time.Time `orm:"type(datetime);index"`
	DisconnectedAt time.Time `orm:"type(datetime);null"`
	BytesIn        int64
	BytesOut       int64
	DurationS      int64
	Status         string `orm:"size(16);index"` // active | closed
}

// TrafficSample is a raw 1-minute-ish snapshot of a client's cumulative
// transfer counters as read from openvpn-status.log. Rolled up into
// TrafficHourly by the retention GC.
type TrafficSample struct {
	Id         int64
	SessionId  int64     `orm:"index"`
	CommonName string    `orm:"size(128);index"`
	VirtualIP  string    `orm:"size(45)"`
	RealIP     string    `orm:"size(45)"`
	BytesIn    int64     // cumulative within the session
	BytesOut   int64     // cumulative within the session
	SampledAt  time.Time `orm:"type(datetime);index"`
}

// TrafficHourly stores per-(user, hour) deltas derived from TrafficSample.
// Kept for OPENVPN_UI_MONITORING_HOURLY_RETENTION_DAYS, then rolled to daily.
type TrafficHourly struct {
	Id            int64
	CommonName    string    `orm:"size(128);index"`
	HourTs        time.Time `orm:"type(datetime);index"` // start of the hour (UTC)
	BytesInDelta  int64
	BytesOutDelta int64
	SessionCount  int
}

// TrafficDaily stores per-(user, day) deltas. Kept indefinitely — the table
// stays small (hundreds of rows per user per year).
type TrafficDaily struct {
	Id            int64
	CommonName    string    `orm:"size(128);index"`
	DayTs         time.Time `orm:"type(date);index"`
	BytesInDelta  int64
	BytesOutDelta int64
	SessionCount  int
}

// --- VpnSession CRUD ---

func (s *VpnSession) Insert() error {
	_, err := orm.NewOrm().Insert(s)
	return err
}

func (s *VpnSession) Update(fields ...string) error {
	_, err := orm.NewOrm().Update(s, fields...)
	return err
}

// FindActiveSession returns the currently active VpnSession for a common name,
// identified by the exact ConnectedAt timestamp reported by OpenVPN.
func FindActiveSession(commonName string, connectedAt time.Time) (*VpnSession, error) {
	var s VpnSession
	err := orm.NewOrm().QueryTable(new(VpnSession)).
		Filter("CommonName", commonName).
		Filter("ConnectedAt", connectedAt).
		Filter("Status", "active").
		One(&s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// FindActiveSessionsByCN returns all active sessions for a user (typically one).
func FindActiveSessionsByCN(commonName string) ([]*VpnSession, error) {
	var out []*VpnSession
	_, err := orm.NewOrm().QueryTable(new(VpnSession)).
		Filter("CommonName", commonName).
		Filter("Status", "active").
		All(&out)
	return out, err
}

// ListActiveSessions returns every session currently marked active.
func ListActiveSessions() ([]*VpnSession, error) {
	var out []*VpnSession
	_, err := orm.NewOrm().QueryTable(new(VpnSession)).
		Filter("Status", "active").
		OrderBy("CommonName", "-ConnectedAt").
		All(&out)
	return out, err
}

// ListRecentSessions returns the N most recent sessions (active or closed).
func ListRecentSessions(limit int) ([]*VpnSession, error) {
	if limit <= 0 {
		limit = 50
	}
	var out []*VpnSession
	_, err := orm.NewOrm().QueryTable(new(VpnSession)).
		OrderBy("-ConnectedAt").
		Limit(limit).
		All(&out)
	return out, err
}

// --- TrafficSample helpers ---

func (t *TrafficSample) Insert() error {
	_, err := orm.NewOrm().Insert(t)
	return err
}

// --- TrafficHourly helpers ---

func (h *TrafficHourly) Insert() error {
	_, err := orm.NewOrm().Insert(h)
	return err
}

// --- TrafficDaily helpers ---

func (d *TrafficDaily) Insert() error {
	_, err := orm.NewOrm().Insert(d)
	return err
}

// InfluxSettings is a singleton row (Id = 1) that overrides the InfluxDB
// values from app.conf. Present only when the operator has saved connection
// details via the Monitor → InfluxDB tab; otherwise the scraper falls back to
// app.conf / env vars.
type InfluxSettings struct {
	Id        int64
	URL       string    `orm:"size(256)"`
	Token     string    `orm:"size(512)"`
	Database  string    `orm:"size(128)"`
	UpdatedAt time.Time `orm:"type(datetime);auto_now"`
}

// GetInfluxSettings returns the singleton row, or nil if none saved.
func GetInfluxSettings() (*InfluxSettings, error) {
	s := &InfluxSettings{Id: 1}
	if err := orm.NewOrm().Read(s); err != nil {
		if err == orm.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return s, nil
}

// SaveInfluxSettings upserts the singleton row. An empty token leaves the
// existing one untouched — lets admins update URL without re-typing secrets.
func SaveInfluxSettings(url, token, database string) error {
	o := orm.NewOrm()
	existing := &InfluxSettings{Id: 1}
	err := o.Read(existing)
	if err == orm.ErrNoRows {
		_, err = o.Insert(&InfluxSettings{Id: 1, URL: url, Token: token, Database: database})
		return err
	}
	if err != nil {
		return err
	}
	existing.URL = url
	existing.Database = database
	if token != "" {
		existing.Token = token
	}
	_, err = o.Update(existing, "URL", "Database", "Token", "UpdatedAt")
	return err
}
