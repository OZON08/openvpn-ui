package controllers

import (
	"crypto/subtle"
	"encoding/json"
	"strconv"
	"time"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"

	"github.com/OZON08/openvpn-ui/models"
	"github.com/OZON08/openvpn-ui/state"
)

// MonitorController renders /monitor — the traffic history UI.
type MonitorController struct {
	BaseController
}

func (c *MonitorController) NestPrepare() {
	if !c.IsLogin {
		c.Ctx.Redirect(302, c.LoginPath())
		return
	}
}

func (c *MonitorController) Get() {
	c.TplName = "monitor.html"
	c.Data["breadcrumbs"] = &BreadCrumbs{Title: "Monitor"}

	active, _ := models.ListActiveSessions()
	recent, _ := models.ListRecentSessions(50)

	c.Data["ActiveSessions"] = active
	c.Data["RecentSessions"] = recent
	c.Data["MonitorEnabled"] = state.Monitor != nil
}

// --- API controllers (mounted under /api/v1/monitor) ---

// APIMonitorSessionsController exposes session lists as JSON.
type APIMonitorSessionsController struct {
	APIBaseController
}

// Get returns active and recent sessions.
// @router / [get]
func (c *APIMonitorSessionsController) Get() {
	active, err := models.ListActiveSessions()
	if err != nil {
		c.ServeJSONError(err.Error())
		return
	}
	limit := 50
	if q := c.GetString("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	recent, err := models.ListRecentSessions(limit)
	if err != nil {
		c.ServeJSONError(err.Error())
		return
	}
	c.ServeJSONData(map[string]interface{}{
		"active": active,
		"recent": recent,
	})
}

// APIMonitorTrafficController returns aggregated per-user traffic.
type APIMonitorTrafficController struct {
	APIBaseController
}

// Get returns hourly and daily totals for a given common_name. Query params:
//   ?cn=<common_name>&range=7d|30d|365d
// @router / [get]
func (c *APIMonitorTrafficController) Get() {
	cn := c.GetString("cn")
	if cn == "" {
		c.ServeJSONError("missing cn parameter")
		return
	}
	rng := c.GetString("range")
	if rng == "" {
		rng = "30d"
	}
	days := parseRangeDays(rng)
	since := time.Now().UTC().AddDate(0, 0, -days)

	o := orm.NewOrm()

	var hourly []models.TrafficHourly
	_, err := o.QueryTable(new(models.TrafficHourly)).
		Filter("CommonName", cn).
		Filter("HourTs__gte", since).
		OrderBy("HourTs").
		All(&hourly)
	if err != nil {
		c.ServeJSONError(err.Error())
		return
	}

	var daily []models.TrafficDaily
	_, err = o.QueryTable(new(models.TrafficDaily)).
		Filter("CommonName", cn).
		Filter("DayTs__gte", since).
		OrderBy("DayTs").
		All(&daily)
	if err != nil {
		c.ServeJSONError(err.Error())
		return
	}

	c.ServeJSONData(map[string]interface{}{
		"common_name": cn,
		"range_days":  days,
		"hourly":      hourly,
		"daily":       daily,
	})
}

func parseRangeDays(r string) int {
	switch r {
	case "1d":
		return 1
	case "7d":
		return 7
	case "30d":
		return 30
	case "90d":
		return 90
	case "365d", "1y":
		return 365
	}
	return 30
}

// APIMonitorHookController receives client-disconnect webhooks from OpenVPN
// and closes the corresponding VpnSession with authoritative byte counts.
type APIMonitorHookController struct {
	APIBaseController
}

// Prepare disables the session/XSRF checks — the hook authenticates via a
// shared secret instead, so it can be called by a shell script without a
// browser session.
func (c *APIMonitorHookController) Prepare() {
	c.EnableXSRF = false
	// Do NOT call BaseController.Prepare() here — that would demand a
	// logged-in session which the script does not have.
}

// Post handles the client-disconnect webhook. It expects JSON:
//   {
//     "common_name": "alice",
//     "real_ip":     "1.2.3.4",
//     "virtual_ip":  "10.0.70.2",
//     "connected_at": 1713700000,
//     "bytes_in":    12345,
//     "bytes_out":   67890,
//     "duration_s":  3600
//   }
// and the header "X-Monitor-Token: <shared secret>".
// @router / [post]
func (c *APIMonitorHookController) Post() {
	if state.Monitor == nil {
		c.ServeJSONError("monitoring disabled")
		return
	}
	expected := state.Monitor.HookToken()
	if expected == "" {
		c.ServeJSONError("hook token not configured")
		return
	}
	got := c.Ctx.Input.Header("X-Monitor-Token")
	if subtle.ConstantTimeCompare([]byte(got), []byte(expected)) != 1 {
		c.ServeJSONError("unauthorized")
		return
	}

	var payload struct {
		CommonName  string `json:"common_name"`
		RealIP      string `json:"real_ip"`
		VirtualIP   string `json:"virtual_ip"`
		ConnectedAt int64  `json:"connected_at"`
		BytesIn     int64  `json:"bytes_in"`
		BytesOut    int64  `json:"bytes_out"`
		DurationS   int64  `json:"duration_s"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil {
		c.ServeJSONError("bad json: " + err.Error())
		return
	}
	if payload.CommonName == "" {
		c.ServeJSONError("missing common_name")
		return
	}

	now := time.Now().UTC()
	var session *models.VpnSession
	if payload.ConnectedAt > 0 {
		connectedAt := time.Unix(payload.ConnectedAt, 0).UTC()
		session, _ = models.FindActiveSession(payload.CommonName, connectedAt)
	}
	if session == nil {
		// Fall back: close the newest active session for this CN.
		active, _ := models.FindActiveSessionsByCN(payload.CommonName)
		if len(active) > 0 {
			session = active[0]
			for _, s := range active[1:] {
				if s.ConnectedAt.After(session.ConnectedAt) {
					session = s
				}
			}
		}
	}
	if session == nil {
		logs.Info("disconnect hook: no active session for %s (creating closed record)", payload.CommonName)
		session = &models.VpnSession{
			CommonName:  payload.CommonName,
			RealIP:      payload.RealIP,
			VirtualIP:   payload.VirtualIP,
			ConnectedAt: now.Add(-time.Duration(payload.DurationS) * time.Second),
			BytesIn:     payload.BytesIn,
			BytesOut:    payload.BytesOut,
			DurationS:   payload.DurationS,
			Status:      "closed",
		}
		session.DisconnectedAt = now
		if err := session.Insert(); err != nil {
			c.ServeJSONError(err.Error())
			return
		}
	} else {
		session.BytesIn = payload.BytesIn
		session.BytesOut = payload.BytesOut
		session.DurationS = payload.DurationS
		session.DisconnectedAt = now
		session.Status = "closed"
		if payload.RealIP != "" {
			session.RealIP = payload.RealIP
		}
		if payload.VirtualIP != "" {
			session.VirtualIP = payload.VirtualIP
		}
		if err := session.Update("BytesIn", "BytesOut", "DurationS", "DisconnectedAt", "Status", "RealIP", "VirtualIP"); err != nil {
			c.ServeJSONError(err.Error())
			return
		}
	}

	state.Monitor.Influx().WriteSession(
		session.CommonName, session.RealIP, session.VirtualIP,
		session.BytesIn, session.BytesOut, session.DurationS, now,
	)

	c.ServeJSONMessage("ok")
}
