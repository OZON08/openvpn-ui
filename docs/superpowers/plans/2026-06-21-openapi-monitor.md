# OpenAPI Monitor Endpoints Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two admin-only API endpoints (`/retention`, `/influx`), apply per-user scoping to the existing sessions/traffic endpoints, expose a Swagger UI page in the sidebar at `/api-docs`, and document all five monitor endpoints in the swagger files.

**Architecture:** All Go changes live in `controllers/monitor.go` (new controllers + scoping) and a new `controllers/apidocs.go` (page controller). The swagger static files (`swagger/swagger.yml` and `swagger/swagger.json`) are hand-edited — no code generation. The Swagger UI iframe page follows the identical pattern as `views/grafana.html`.

**Tech Stack:** Go 1.21+, beego v2, standard library only. Go binary: `/usr/local/go/bin/go` — run as `export PATH=$PATH:/usr/local/go/bin && go build ./...`

## Global Constraints

- Package `controllers` throughout — no new packages
- All API error responses use the existing `JSONResponse` envelope: `{"status":"error","message":"..."}`
- All API success responses use `ServeJSONData(data)` which wraps in `{"status":"success","data":...}`
- Admin-check failures return HTTP 403 and call `c.StopRun()` to prevent the action from running
- Non-admin cert scoping uses `models.CertsForUser(c.Userinfo.Id)` — same pattern as `MainController.Get()`
- Swagger files document field names exactly as Go serializes them (PascalCase for structs without json tags; snake_case for explicit maps)
- `swagger/swagger.json` is the authoritative file read by the Swagger UI; `swagger/swagger.yml` is the human-readable twin — both must be kept in sync

---

### Task 1: New admin API endpoints — retention + influx

**Files:**
- Modify: `controllers/monitor.go` (append after the existing `APIMonitorHookController`)
- Modify: `routers/router.go` (register two new NSRouter entries)

**Interfaces:**
- Consumes: `loadRetentionStages() []RetentionStage` and `loadInfluxStatus() (*InfluxStatusView, *InfluxConfigView)` — both already defined in `controllers/monitor.go`
- Consumes: `JSONResponse` struct from `controllers/api-base.go` (same package)
- Produces: `APIMonitorRetentionController` and `APIMonitorInfluxController` — used by Task 1's router registration only

---

- [ ] **Step 1: Append the two new controllers to `controllers/monitor.go`**

Add the following block at the very end of `controllers/monitor.go` (after the closing `}` of `APIMonitorHookController.Post`):

```go
// APIMonitorRetentionController exposes retention pipeline stats as JSON.
// Admin-only.
type APIMonitorRetentionController struct {
	APIBaseController
}

func (c *APIMonitorRetentionController) NestPrepare() {
	if !c.IsLogin {
		c.ServeJSONError("You are not authorized")
		c.StopRun()
		return
	}
	if c.Userinfo == nil || !c.Userinfo.IsAdmin {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = &JSONResponse{Status: "error", Message: "admin privileges required"}
		c.ServeJSON()
		c.StopRun()
	}
}

// Get returns retention stage stats for TrafficSample, TrafficHourly, and TrafficDaily.
func (c *APIMonitorRetentionController) Get() {
	c.ServeJSONData(loadRetentionStages())
}

// APIMonitorInfluxController exposes InfluxDB writer status as JSON.
// Admin-only.
type APIMonitorInfluxController struct {
	APIBaseController
}

func (c *APIMonitorInfluxController) NestPrepare() {
	if !c.IsLogin {
		c.ServeJSONError("You are not authorized")
		c.StopRun()
		return
	}
	if c.Userinfo == nil || !c.Userinfo.IsAdmin {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = &JSONResponse{Status: "error", Message: "admin privileges required"}
		c.ServeJSON()
		c.StopRun()
	}
}

// Get returns InfluxDB writer status and config. Returns enabled=false with zero
// counters when InfluxDB is disabled or monitoring is not running.
func (c *APIMonitorInfluxController) Get() {
	status, config := loadInfluxStatus()
	if status == nil || config == nil {
		c.ServeJSONData(map[string]interface{}{
			"enabled":     false,
			"url":         "",
			"database":    "",
			"buffered":    0,
			"flushed_24h": 0,
			"errors_24h":  0,
		})
		return
	}
	c.ServeJSONData(map[string]interface{}{
		"enabled":     config.Enabled,
		"url":         config.URL,
		"database":    config.Database,
		"buffered":    status.Buffered,
		"flushed_24h": status.Flushed24h,
		"errors_24h":  status.Errors24h,
	})
}
```

- [ ] **Step 2: Register the two new routes in `routers/router.go`**

The `/monitor` namespace currently reads:

```go
		web.NSNamespace("/monitor",
			web.NSRouter("/sessions", &controllers.APIMonitorSessionsController{}, "get:Get"),
			web.NSRouter("/traffic", &controllers.APIMonitorTrafficController{}, "get:Get"),
			web.NSRouter("/disconnect", &controllers.APIMonitorHookController{}, "post:Post"),
		),
```

Change it to:

```go
		web.NSNamespace("/monitor",
			web.NSRouter("/sessions", &controllers.APIMonitorSessionsController{}, "get:Get"),
			web.NSRouter("/traffic", &controllers.APIMonitorTrafficController{}, "get:Get"),
			web.NSRouter("/disconnect", &controllers.APIMonitorHookController{}, "post:Post"),
			web.NSRouter("/retention", &controllers.APIMonitorRetentionController{}, "get:Get"),
			web.NSRouter("/influx", &controllers.APIMonitorInfluxController{}, "get:Get"),
		),
```

- [ ] **Step 3: Build to verify no compilation errors**

```bash
export PATH=$PATH:/usr/local/go/bin && go build ./...
```

Expected: no output (clean build).

- [ ] **Step 4: Commit**

```bash
git add controllers/monitor.go routers/router.go
git commit -m "feat: add admin-only retention and influx API endpoints"
```

---

### Task 2: Non-admin scoping for sessions and traffic

**Files:**
- Modify: `controllers/monitor.go` — two method bodies changed

**Interfaces:**
- Consumes: `models.CertsForUser(userID int64) ([]string, error)` from `models/user_certificate.go`
- Consumes: `models.VpnSession` struct (field `CommonName string`)

---

- [ ] **Step 1: Add cert scoping to `APIMonitorSessionsController.Get()`**

The current `Get()` body is:

```go
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
```

Replace it with:

```go
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
	if !c.Userinfo.IsAdmin {
		allowed, _ := models.CertsForUser(c.Userinfo.Id)
		allowSet := make(map[string]bool, len(allowed))
		for _, n := range allowed {
			allowSet[n] = true
		}
		filtered := make([]*models.VpnSession, 0)
		for _, s := range active {
			if allowSet[s.CommonName] {
				filtered = append(filtered, s)
			}
		}
		active = filtered
		filteredRecent := make([]*models.VpnSession, 0)
		for _, s := range recent {
			if allowSet[s.CommonName] {
				filteredRecent = append(filteredRecent, s)
			}
		}
		recent = filteredRecent
	}
	c.ServeJSONData(map[string]interface{}{
		"active": active,
		"recent": recent,
	})
}
```

- [ ] **Step 2: Add cn ownership check to `APIMonitorTrafficController.Get()`**

The current `Get()` starts with:

```go
func (c *APIMonitorTrafficController) Get() {
	cn := c.GetString("cn")
	if cn == "" {
		c.ServeJSONError("missing cn parameter")
		return
	}
	rng := c.GetString("range")
```

Replace that opening block with:

```go
func (c *APIMonitorTrafficController) Get() {
	cn := c.GetString("cn")
	if cn == "" {
		c.ServeJSONError("missing cn parameter")
		return
	}
	if !c.Userinfo.IsAdmin {
		allowed, _ := models.CertsForUser(c.Userinfo.Id)
		found := false
		for _, n := range allowed {
			if n == cn {
				found = true
				break
			}
		}
		if !found {
			c.ServeJSONError("access denied")
			return
		}
	}
	rng := c.GetString("range")
```

Leave everything from `rng := c.GetString("range")` onward unchanged.

- [ ] **Step 3: Build to verify no compilation errors**

```bash
export PATH=$PATH:/usr/local/go/bin && go build ./...
```

Expected: no output (clean build).

- [ ] **Step 4: Commit**

```bash
git add controllers/monitor.go
git commit -m "feat: scope sessions and traffic API to assigned certs for non-admins"
```

---

### Task 3: API docs page — controller, view, sidebar, routing

**Files:**
- Create: `controllers/apidocs.go`
- Create: `views/api-docs.html`
- Modify: `views/common/sidebar.html`
- Modify: `routers/router.go`

**Interfaces:**
- Produces: `APIDocsController` registered at `/api-docs` — renders `views/api-docs.html`
- Produces: sidebar entry "API" visible to all logged-in users, active when `RouterPattern == "/api-docs"`

---

- [ ] **Step 1: Create `controllers/apidocs.go`**

```go
package controllers

// APIDocsController renders the Swagger UI embedded in a full-screen iframe.
type APIDocsController struct {
	BaseController
}

func (c *APIDocsController) NestPrepare() {
	if !c.IsLogin {
		c.Ctx.Redirect(302, c.LoginPath())
		return
	}
	c.Data["breadcrumbs"] = &BreadCrumbs{Title: "API Docs"}
}

func (c *APIDocsController) Get() {
	c.TplName = "api-docs.html"
}
```

- [ ] **Step 2: Create `views/api-docs.html`**

```html
{{ template "layout/base.html" . }}
{{define "head"}}
<title>OpenVPN - API Docs</title>
<style>
  #api-docs-frame {
    position: fixed;
    top: 57px;
    left: 250px;
    width: calc(100% - 250px);
    height: calc(100vh - 57px);
    border: none;
    z-index: 100;
  }
  .sidebar-collapse #api-docs-frame {
    left: 60px;
    width: calc(100% - 60px);
  }
</style>
{{end}}

{{define "body"}}
<iframe id="api-docs-frame" src="/api/docs/"></iframe>
{{end}}
```

- [ ] **Step 3: Add the "API" sidebar entry in `views/common/sidebar.html`**

The current sidebar has this block (lines 30–36):

```html
        <li class="nav-item">
          <a href="{{urlfor "LogsController.Get"}}" class="nav-link {{if compare .RouterPattern "/logs"}}active{{end}}">
            <i class="nav-icon fa fa-file-text-o"></i>
            <p>Logs</p>
          </a>
        </li>

        {{if .Userinfo.IsAdmin}}
```

Change it to:

```html
        <li class="nav-item">
          <a href="{{urlfor "LogsController.Get"}}" class="nav-link {{if compare .RouterPattern "/logs"}}active{{end}}">
            <i class="nav-icon fa fa-file-text-o"></i>
            <p>Logs</p>
          </a>
        </li>

        <li class="nav-item">
          <a href="{{urlfor "APIDocsController.Get"}}" class="nav-link {{if compare .RouterPattern "/api-docs"}}active{{end}}">
            <i class="nav-icon fa fa-code"></i>
            <p>API</p>
          </a>
        </li>

        {{if .Userinfo.IsAdmin}}
```

- [ ] **Step 4: Update `routers/router.go`**

Change the static path from `/swagger` to `/api/docs`:

```go
// Before:
web.SetStaticPath("/swagger", "swagger")

// After:
web.SetStaticPath("/api/docs", "swagger")
```

Add the new page route after the existing `/monitor` and `/grafana` routes:

```go
web.Router("/api-docs", &controllers.APIDocsController{})
```

Place it directly after `web.Router("/grafana/*", ...)`.

- [ ] **Step 5: Build to verify no compilation errors**

```bash
export PATH=$PATH:/usr/local/go/bin && go build ./...
```

Expected: no output (clean build).

- [ ] **Step 6: Commit**

```bash
git add controllers/apidocs.go views/api-docs.html views/common/sidebar.html routers/router.go
git commit -m "feat: add API docs page with Swagger UI iframe and sidebar entry"
```

---

### Task 4: Swagger documentation for all monitor endpoints

**Files:**
- Rewrite: `swagger/swagger.yml`
- Rewrite: `swagger/swagger.json`

Both files must stay in sync — same content, different format. The Swagger UI reads `swagger.json`.

**Interfaces:**
- Consumes: nothing from earlier tasks (static documentation only)

---

- [ ] **Step 1: Rewrite `swagger/swagger.yml`**

Replace the entire file with:

```yaml
swagger: "2.0"
info:
  title: OpenVPN API
  description: REST API allows you to control and monitor your OpenVPN server
  version: 1.0.0.0-dev
  contact:
    email: adam.walach@gmail.com
basePath: /api/v1
securityDefinitions:
  MonitorToken:
    type: apiKey
    in: header
    name: X-Monitor-Token
paths:
  /session/:
    get:
      tags:
      - session
      description: List vpn sessions
      operationId: APISessionController.list
      responses:
        "200":
          description: request success
        "400":
          description: request failure
    delete:
      tags:
      - session
      description: Delete (kill) session
      operationId: APISessionController.Kill
      parameters:
      - in: body
        name: body
        description: CommonName of client to kill
        required: true
        schema:
          $ref: '#/definitions/controllers.KillParams'
      responses:
        "200":
          description: request success
        "400":
          description: request failure
  /signal/:
    post:
      tags:
      - signal
      description: Sends signal to OpenVPN daemon
      operationId: APISignalController.Send signal
      parameters:
      - in: body
        name: body
        description: Signal to send
        required: true
        schema:
          $ref: '#/definitions/controllers.SignalParams'
      responses:
        "200":
          description: request success
        "400":
          description: request failure
  /sysload/:
    get:
      tags:
      - sysload
      description: Shows OS stats
      operationId: APISysloadController.Get system info
      responses:
        "200":
          description: request success
        "400":
          description: request failure
  /monitor/sessions:
    get:
      tags:
      - monitor
      description: >
        List active and recent VPN sessions.
        Non-admin users see only sessions whose CommonName matches their assigned certificates.
      operationId: APIMonitorSessionsController.Get
      parameters:
      - in: query
        name: limit
        description: Maximum number of recent sessions to return (1–500, default 50)
        type: integer
      responses:
        "200":
          description: success
          schema:
            properties:
              status:
                type: string
              data:
                properties:
                  active:
                    type: array
                    items:
                      $ref: '#/definitions/models.VpnSession'
                  recent:
                    type: array
                    items:
                      $ref: '#/definitions/models.VpnSession'
        "400":
          description: error
  /monitor/traffic:
    get:
      tags:
      - monitor
      description: >
        Per-user traffic history (hourly and daily deltas).
        Non-admin users may only query a cn that is in their assigned certificates.
      operationId: APIMonitorTrafficController.Get
      parameters:
      - in: query
        name: cn
        description: Common name to query (required)
        required: true
        type: string
      - in: query
        name: range
        description: "Time range: 1d | 7d | 30d | 90d | 365d (default: 30d)"
        type: string
      responses:
        "200":
          description: success
          schema:
            properties:
              status:
                type: string
              data:
                properties:
                  common_name:
                    type: string
                  range_days:
                    type: integer
                  hourly:
                    type: array
                    items:
                      $ref: '#/definitions/models.TrafficHourly'
                  daily:
                    type: array
                    items:
                      $ref: '#/definitions/models.TrafficDaily'
        "400":
          description: error (includes access denied for non-admins querying other users)
  /monitor/disconnect:
    post:
      tags:
      - monitor
      description: >
        Client-disconnect webhook called by the OpenVPN server hook script.
        Requires the X-Monitor-Token header matching the configured shared secret.
        Does not require a login session.
      operationId: APIMonitorHookController.Post
      security:
      - MonitorToken: []
      parameters:
      - in: body
        name: body
        required: true
        schema:
          $ref: '#/definitions/controllers.DisconnectPayload'
      responses:
        "200":
          description: success
        "400":
          description: error (token missing/invalid, monitoring disabled, bad JSON)
  /monitor/retention:
    get:
      tags:
      - monitor
      description: "Retention pipeline stats: row counts and policies for each aggregation stage. Admin only."
      operationId: APIMonitorRetentionController.Get
      responses:
        "200":
          description: success
          schema:
            properties:
              status:
                type: string
              data:
                type: array
                items:
                  $ref: '#/definitions/controllers.RetentionStage'
        "400":
          description: error
        "403":
          description: admin privileges required
  /monitor/influx:
    get:
      tags:
      - monitor
      description: "InfluxDB writer status and configuration. Admin only. Returns enabled=false when InfluxDB is disabled."
      operationId: APIMonitorInfluxController.Get
      responses:
        "200":
          description: success
          schema:
            properties:
              status:
                type: string
              data:
                $ref: '#/definitions/controllers.InfluxStatus'
        "400":
          description: error
        "403":
          description: admin privileges required
definitions:
  controllers.KillParams:
    title: KillParams
    type: object
    properties:
      cname:
        type: string
  controllers.SignalParams:
    title: SignalParams
    type: object
    properties:
      sname:
        type: string
  controllers.DisconnectPayload:
    title: DisconnectPayload
    type: object
    required:
    - common_name
    properties:
      common_name:
        type: string
      real_ip:
        type: string
      virtual_ip:
        type: string
      connected_at:
        type: integer
        description: Unix timestamp (seconds)
      bytes_in:
        type: integer
      bytes_out:
        type: integer
      duration_s:
        type: integer
  controllers.RetentionStage:
    title: RetentionStage
    type: object
    properties:
      Name:
        type: string
      Granularity:
        type: string
      Rows:
        type: integer
      Size:
        type: string
      Policy:
        type: string
  controllers.InfluxStatus:
    title: InfluxStatus
    type: object
    properties:
      enabled:
        type: boolean
      url:
        type: string
      database:
        type: string
      buffered:
        type: integer
      flushed_24h:
        type: integer
      errors_24h:
        type: integer
  models.VpnSession:
    title: VpnSession
    type: object
    properties:
      Id:
        type: integer
      CommonName:
        type: string
      RealIP:
        type: string
      VirtualIP:
        type: string
      ConnectedAt:
        type: string
        format: date-time
      DisconnectedAt:
        type: string
        format: date-time
      BytesIn:
        type: integer
      BytesOut:
        type: integer
      DurationS:
        type: integer
      Status:
        type: string
        description: "active | closed"
  models.TrafficHourly:
    title: TrafficHourly
    type: object
    properties:
      Id:
        type: integer
      CommonName:
        type: string
      HourTs:
        type: string
        format: date-time
      BytesInDelta:
        type: integer
      BytesOutDelta:
        type: integer
      SessionCount:
        type: integer
  models.TrafficDaily:
    title: TrafficDaily
    type: object
    properties:
      Id:
        type: integer
      CommonName:
        type: string
      DayTs:
        type: string
        format: date
      BytesInDelta:
        type: integer
      BytesOutDelta:
        type: integer
      SessionCount:
        type: integer
tags:
- name: session
  description: |
    APISessionController manages vpn sessions
- name: sysload
  description: |
    APISysloadController provides system information
- name: signal
  description: |
    APISignalController sends signals to OpenVPN daemon
- name: monitor
  description: |
    Monitor controllers expose VPN session data, traffic history, retention stats, and InfluxDB status
```

- [ ] **Step 2: Rewrite `swagger/swagger.json`**

Replace the entire file with:

```json
{
    "swagger": "2.0",
    "info": {
        "title": "OpenVPN API",
        "description": "REST API allows you to control and monitor your OpenVPN server",
        "version": "1.0.0.0-dev",
        "contact": {
            "email": "adam.walach@gmail.com"
        }
    },
    "basePath": "/api/v1",
    "securityDefinitions": {
        "MonitorToken": {
            "type": "apiKey",
            "in": "header",
            "name": "X-Monitor-Token"
        }
    },
    "paths": {
        "/session/": {
            "get": {
                "tags": ["session"],
                "description": "List vpn sessions",
                "operationId": "APISessionController.list",
                "responses": {
                    "200": { "description": "request success" },
                    "400": { "description": "request failure" }
                }
            },
            "delete": {
                "tags": ["session"],
                "description": "Delete (kill) session",
                "operationId": "APISessionController.Kill",
                "parameters": [
                    {
                        "in": "body",
                        "name": "body",
                        "description": "CommonName of client to kill",
                        "required": true,
                        "schema": { "$ref": "#/definitions/controllers.KillParams" }
                    }
                ],
                "responses": {
                    "200": { "description": "request success" },
                    "400": { "description": "request failure" }
                }
            }
        },
        "/signal/": {
            "post": {
                "tags": ["signal"],
                "description": "Sends signal to OpenVPN daemon",
                "operationId": "APISignalController.Send signal",
                "parameters": [
                    {
                        "in": "body",
                        "name": "body",
                        "description": "Signal to send",
                        "required": true,
                        "schema": { "$ref": "#/definitions/controllers.SignalParams" }
                    }
                ],
                "responses": {
                    "200": { "description": "request success" },
                    "400": { "description": "request failure" }
                }
            }
        },
        "/sysload/": {
            "get": {
                "tags": ["sysload"],
                "description": "Shows OS stats",
                "operationId": "APISysloadController.Get system info",
                "responses": {
                    "200": { "description": "request success" },
                    "400": { "description": "request failure" }
                }
            }
        },
        "/monitor/sessions": {
            "get": {
                "tags": ["monitor"],
                "description": "List active and recent VPN sessions. Non-admin users see only sessions whose CommonName matches their assigned certificates.",
                "operationId": "APIMonitorSessionsController.Get",
                "parameters": [
                    {
                        "in": "query",
                        "name": "limit",
                        "description": "Maximum number of recent sessions to return (1-500, default 50)",
                        "type": "integer"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "success",
                        "schema": {
                            "properties": {
                                "status": { "type": "string" },
                                "data": {
                                    "properties": {
                                        "active": {
                                            "type": "array",
                                            "items": { "$ref": "#/definitions/models.VpnSession" }
                                        },
                                        "recent": {
                                            "type": "array",
                                            "items": { "$ref": "#/definitions/models.VpnSession" }
                                        }
                                    }
                                }
                            }
                        }
                    },
                    "400": { "description": "error" }
                }
            }
        },
        "/monitor/traffic": {
            "get": {
                "tags": ["monitor"],
                "description": "Per-user traffic history (hourly and daily deltas). Non-admin users may only query a cn that is in their assigned certificates.",
                "operationId": "APIMonitorTrafficController.Get",
                "parameters": [
                    {
                        "in": "query",
                        "name": "cn",
                        "description": "Common name to query (required)",
                        "required": true,
                        "type": "string"
                    },
                    {
                        "in": "query",
                        "name": "range",
                        "description": "Time range: 1d | 7d | 30d | 90d | 365d (default: 30d)",
                        "type": "string"
                    }
                ],
                "responses": {
                    "200": {
                        "description": "success",
                        "schema": {
                            "properties": {
                                "status": { "type": "string" },
                                "data": {
                                    "properties": {
                                        "common_name": { "type": "string" },
                                        "range_days": { "type": "integer" },
                                        "hourly": {
                                            "type": "array",
                                            "items": { "$ref": "#/definitions/models.TrafficHourly" }
                                        },
                                        "daily": {
                                            "type": "array",
                                            "items": { "$ref": "#/definitions/models.TrafficDaily" }
                                        }
                                    }
                                }
                            }
                        }
                    },
                    "400": { "description": "error (includes access denied for non-admins querying other users)" }
                }
            }
        },
        "/monitor/disconnect": {
            "post": {
                "tags": ["monitor"],
                "description": "Client-disconnect webhook called by the OpenVPN server hook script. Requires the X-Monitor-Token header. Does not require a login session.",
                "operationId": "APIMonitorHookController.Post",
                "security": [{ "MonitorToken": [] }],
                "parameters": [
                    {
                        "in": "body",
                        "name": "body",
                        "required": true,
                        "schema": { "$ref": "#/definitions/controllers.DisconnectPayload" }
                    }
                ],
                "responses": {
                    "200": { "description": "success" },
                    "400": { "description": "error (token missing/invalid, monitoring disabled, bad JSON)" }
                }
            }
        },
        "/monitor/retention": {
            "get": {
                "tags": ["monitor"],
                "description": "Retention pipeline stats: row counts and policies for each aggregation stage. Admin only.",
                "operationId": "APIMonitorRetentionController.Get",
                "responses": {
                    "200": {
                        "description": "success",
                        "schema": {
                            "properties": {
                                "status": { "type": "string" },
                                "data": {
                                    "type": "array",
                                    "items": { "$ref": "#/definitions/controllers.RetentionStage" }
                                }
                            }
                        }
                    },
                    "400": { "description": "error" },
                    "403": { "description": "admin privileges required" }
                }
            }
        },
        "/monitor/influx": {
            "get": {
                "tags": ["monitor"],
                "description": "InfluxDB writer status and configuration. Admin only. Returns enabled=false when InfluxDB is disabled.",
                "operationId": "APIMonitorInfluxController.Get",
                "responses": {
                    "200": {
                        "description": "success",
                        "schema": {
                            "properties": {
                                "status": { "type": "string" },
                                "data": { "$ref": "#/definitions/controllers.InfluxStatus" }
                            }
                        }
                    },
                    "400": { "description": "error" },
                    "403": { "description": "admin privileges required" }
                }
            }
        }
    },
    "definitions": {
        "controllers.KillParams": {
            "title": "KillParams",
            "type": "object",
            "properties": {
                "cname": { "type": "string" }
            }
        },
        "controllers.SignalParams": {
            "title": "SignalParams",
            "type": "object",
            "properties": {
                "sname": { "type": "string" }
            }
        },
        "controllers.DisconnectPayload": {
            "title": "DisconnectPayload",
            "type": "object",
            "required": ["common_name"],
            "properties": {
                "common_name": { "type": "string" },
                "real_ip": { "type": "string" },
                "virtual_ip": { "type": "string" },
                "connected_at": { "type": "integer", "description": "Unix timestamp (seconds)" },
                "bytes_in": { "type": "integer" },
                "bytes_out": { "type": "integer" },
                "duration_s": { "type": "integer" }
            }
        },
        "controllers.RetentionStage": {
            "title": "RetentionStage",
            "type": "object",
            "properties": {
                "Name": { "type": "string" },
                "Granularity": { "type": "string" },
                "Rows": { "type": "integer" },
                "Size": { "type": "string" },
                "Policy": { "type": "string" }
            }
        },
        "controllers.InfluxStatus": {
            "title": "InfluxStatus",
            "type": "object",
            "properties": {
                "enabled": { "type": "boolean" },
                "url": { "type": "string" },
                "database": { "type": "string" },
                "buffered": { "type": "integer" },
                "flushed_24h": { "type": "integer" },
                "errors_24h": { "type": "integer" }
            }
        },
        "models.VpnSession": {
            "title": "VpnSession",
            "type": "object",
            "properties": {
                "Id": { "type": "integer" },
                "CommonName": { "type": "string" },
                "RealIP": { "type": "string" },
                "VirtualIP": { "type": "string" },
                "ConnectedAt": { "type": "string", "format": "date-time" },
                "DisconnectedAt": { "type": "string", "format": "date-time" },
                "BytesIn": { "type": "integer" },
                "BytesOut": { "type": "integer" },
                "DurationS": { "type": "integer" },
                "Status": { "type": "string", "description": "active | closed" }
            }
        },
        "models.TrafficHourly": {
            "title": "TrafficHourly",
            "type": "object",
            "properties": {
                "Id": { "type": "integer" },
                "CommonName": { "type": "string" },
                "HourTs": { "type": "string", "format": "date-time" },
                "BytesInDelta": { "type": "integer" },
                "BytesOutDelta": { "type": "integer" },
                "SessionCount": { "type": "integer" }
            }
        },
        "models.TrafficDaily": {
            "title": "TrafficDaily",
            "type": "object",
            "properties": {
                "Id": { "type": "integer" },
                "CommonName": { "type": "string" },
                "DayTs": { "type": "string", "format": "date" },
                "BytesInDelta": { "type": "integer" },
                "BytesOutDelta": { "type": "integer" },
                "SessionCount": { "type": "integer" }
            }
        }
    },
    "tags": [
        { "name": "session", "description": "APISessionController manages vpn sessions\n" },
        { "name": "sysload", "description": "APISysloadController provides system information\n" },
        { "name": "signal", "description": "APISignalController sends signals to OpenVPN daemon\n" },
        { "name": "monitor", "description": "Monitor controllers expose VPN session data, traffic history, retention stats, and InfluxDB status\n" }
    ]
}
```

- [ ] **Step 3: Build to verify no compilation errors**

```bash
export PATH=$PATH:/usr/local/go/bin && go build ./...
```

Expected: no output.

- [ ] **Step 4: Commit**

```bash
git add swagger/swagger.yml swagger/swagger.json
git commit -m "docs: add all monitor endpoints to OpenAPI swagger spec"
```
