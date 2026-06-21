# OpenAPI Monitor Endpoints Design

## Goal

Add two missing API endpoints (`/api/v1/monitor/retention`, `/api/v1/monitor/influx`), apply
per-user scoping to the three existing monitor endpoints, move the Swagger UI from `/swagger`
to `/api/docs`, document all five monitor endpoints in `swagger.yml` and `swagger.json`, and
expose the Swagger UI as a full-screen iframe page in the sidebar under the menu item "API".

## Scope

### New endpoints

| Endpoint | Method | Auth |
|---|---|---|
| `/api/v1/monitor/retention` | GET | Login + Admin |
| `/api/v1/monitor/influx` | GET | Login + Admin |

### Existing endpoints — auth / scoping changes

| Endpoint | Method | Auth | Change |
|---|---|---|---|
| `/api/v1/monitor/sessions` | GET | Login | Non-admin: filter to assigned certs only |
| `/api/v1/monitor/traffic` | GET | Login | Non-admin: reject `?cn=` not in assigned certs |
| `/api/v1/monitor/disconnect` | POST | `X-Monitor-Token` header | No change |

### Swagger UI path

`web.SetStaticPath("/swagger", "swagger")` → `web.SetStaticPath("/api/docs", "swagger")` in
`routers/router.go`. Static files in `swagger/` are unchanged.

### API docs page (new)

A new page at `/api-docs` renders a full-screen iframe pointing to `/api/docs/`. Accessible
to all logged-in users (non-admins can still read the API docs, even if some endpoints are
admin-only). A new sidebar entry "API" (with a `fa-code` icon) appears for all logged-in
users, placed below the Monitor entry. Same full-screen iframe pattern as `grafana.html`.

## Out of Scope

- Auto-generation via `bee generate docs` (manual edit is consistent with the rest of the file)
- Cookie/session-based `securityDefinitions` (beego sessions are not swagger-standardised)
- New UI pages
- Changes to `/monitor` HTML controller

---

## Architecture

All changes are confined to three files:

| File | Change |
|---|---|
| `controllers/monitor.go` | Add `APIMonitorRetentionController`, `APIMonitorInfluxController`; add scoping to `APIMonitorSessionsController.Get` and `APIMonitorTrafficController.Get` |
| `routers/router.go` | Register two new routes; change static path to `/api/docs` |
| `swagger/swagger.yml` + `swagger/swagger.json` | Add five monitor paths + five schema definitions |

No new files, no new packages.

---

## Endpoint Details

### `GET /api/v1/monitor/retention` (new)

**Auth:** login required + `c.Userinfo.IsAdmin` — returns `403` JSON otherwise.

**When monitoring is disabled** (`state.Monitor == nil`): still responds normally — the retention
table counts come from SQLite directly, not from the monitor runtime.

**Response:**
```json
{
  "status": "success",
  "data": [
    { "name": "TrafficSample", "granularity": "per-scrape (~1 min)", "rows": 1234, "policy": "30 d (then rolled up)" },
    { "name": "TrafficHourly", "granularity": "hourly",              "rows": 720,  "policy": "365 d (then rolled up)" },
    { "name": "TrafficDaily",  "granularity": "daily",               "rows": 90,   "policy": "kept indefinitely" }
  ]
}
```

Reuses the existing `loadRetentionStages()` helper — no new DB logic.

---

### `GET /api/v1/monitor/influx` (new)

**Auth:** login required + `c.Userinfo.IsAdmin` — returns `403` JSON otherwise.

**When InfluxDB is disabled or monitor is nil:** returns `{"enabled": false}` with zero counters
— no error, just a disabled state.

**Response (enabled):**
```json
{
  "status": "success",
  "data": {
    "enabled": true,
    "url": "http://influxdb:8181",
    "database": "openvpn",
    "buffered": 42,
    "flushed_24h": 1440,
    "errors_24h": 0
  }
}
```

Reuses the existing `loadInfluxStatus()` helper.

---

### `GET /api/v1/monitor/sessions` (existing — scoping added)

**Auth:** login required.

**Query params:**
- `limit` — integer 1–500, default 50.

**Scoping:** if `!c.Userinfo.IsAdmin`, call `models.CertsForUser(c.Userinfo.Id)` and filter
both `active` and `recent` lists to entries whose `CommonName` is in the assigned set. Same
pattern as `MainController.Get()`.

**Response:**
```json
{
  "status": "success",
  "data": {
    "active": [ /* VpnSession */ ],
    "recent": [ /* VpnSession */ ]
  }
}
```

---

### `GET /api/v1/monitor/traffic` (existing — scoping added)

**Auth:** login required.

**Query params:**
- `cn` — common name (required).
- `range` — `1d | 7d | 30d | 90d | 365d`, default `30d`.

**Scoping:** if `!c.Userinfo.IsAdmin`, verify `cn` is in `models.CertsForUser(c.Userinfo.Id)`.
If not, return `400` `"access denied"`.

**Response:**
```json
{
  "status": "success",
  "data": {
    "common_name": "alice",
    "range_days": 30,
    "hourly": [ /* TrafficHourly */ ],
    "daily":  [ /* TrafficDaily */ ]
  }
}
```

---

### `POST /api/v1/monitor/disconnect` (existing — no change)

**Auth:** `X-Monitor-Token` header (shared secret). No session.

**Body:**
```json
{
  "common_name": "alice",
  "real_ip": "1.2.3.4",
  "virtual_ip": "10.0.70.2",
  "connected_at": 1713700000,
  "bytes_in": 12345,
  "bytes_out": 67890,
  "duration_s": 3600
}
```

**Response:** `{ "status": "success", "message": "ok" }`

---

## Schema Definitions (swagger)

Five new `definitions` entries added to both `swagger.yml` and `swagger.json`:

| Name | Fields |
|---|---|
| `VpnSession` | id (int), common_name, real_ip, virtual_ip, connected_at (datetime), disconnected_at (datetime), bytes_in (int), bytes_out (int), duration_s (int), status |
| `TrafficHourly` | id (int), common_name, hour_ts (datetime), bytes_in_delta (int), bytes_out_delta (int), session_count (int) |
| `TrafficDaily` | id (int), common_name, day_ts (date), bytes_in_delta (int), bytes_out_delta (int), session_count (int) |
| `RetentionStage` | name, granularity, rows (int), policy |
| `InfluxStatus` | enabled (bool), url, database, buffered (int), flushed_24h (int), errors_24h (int) |

Security: `X-Monitor-Token` documented as `apiKey` in header, applied only to
`/monitor/disconnect`.

---

## JSON Response Envelope

All endpoints wrap responses in the existing `JSONResponse` envelope:

```json
{ "status": "success|error", "message": "...", "data": { ... } }
```

Error responses use HTTP 400 (existing pattern). Admin-check failures use HTTP 403.

---

## Files Changed

1. **`controllers/monitor.go`**
   - Add `APIMonitorRetentionController.Get()` — admin check + `loadRetentionStages()`
   - Add `APIMonitorInfluxController.Get()` — admin check + `loadInfluxStatus()`
   - `APIMonitorSessionsController.Get()` — add non-admin cert filter
   - `APIMonitorTrafficController.Get()` — add non-admin `cn` ownership check

2. **`controllers/apidocs.go`** (new) — `APIDocsController` with `NestPrepare` (login required) + `Get()` rendering `api-docs.html`

3. **`routers/router.go`**
   - `web.SetStaticPath("/api/docs", "swagger")`
   - `web.Router("/api-docs", &controllers.APIDocsController{})`
   - `web.NSRouter("/retention", &controllers.APIMonitorRetentionController{}, "get:Get")`
   - `web.NSRouter("/influx", &controllers.APIMonitorInfluxController{}, "get:Get")`

4. **`views/api-docs.html`** (new) — full-screen iframe to `/api/docs/`, same structure as `grafana.html`

5. **`views/common/sidebar.html`** — add "API" menu entry (all logged-in users, `fa-code` icon, below Monitor)

6. **`swagger/swagger.yml`** — add 5 paths + 5 definitions + apiKey securityDefinition
7. **`swagger/swagger.json`** — same, JSON format
