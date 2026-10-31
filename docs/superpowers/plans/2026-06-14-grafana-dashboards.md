# Grafana Dashboards — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Three provisioned Grafana dashboards (auto-loaded at container start) covering live connections, traffic history with long-term monthly/yearly view, and per-user drill-down. No manual import needed.

**Architecture:** Grafana provisioning via a new `server/grafana/provisioning/` directory mounted read-only into the container. One datasource YAML registers the InfluxDB v3 connection with a fixed UID that all dashboard JSONs reference. Dashboards are stored as JSON and picked up automatically.

**Data sources in InfluxDB (`openvpn` database):**
- `openvpn_traffic` — tags: `common_name`, `virtual_ip`, `real_ip`; fields: `bytes_in`, `bytes_out` (cumulative per session); written every ~60 s per active client
- `openvpn_session` — tags: `common_name`, `status`; fields: `bytes_in`, `bytes_out`, `duration_s`, `real_ip`, `virtual_ip`; written on disconnect

**Tech Stack:** Grafana (latest), InfluxDB v3 Core (SQL via DataFusion), Docker Compose volume provisioning.

**Note on time macros:** Queries use `$__timeFilter(time)` (Grafana InfluxDB plugin SQL macro). If the plugin expands it differently, replace with `time >= to_timestamp(${__from:date:seconds}) AND time <= to_timestamp(${__to:date:seconds})`.

---

### Task 1: Provisioning infrastructure

**Files:**
- Create: `server/grafana/provisioning/datasources/influxdb.yaml`
- Create: `server/grafana/provisioning/dashboards/provider.yaml`
- Modify: `server/docker-compose.yml` — add volume + env var to `openvpn-grafana`

- [ ] **Step 1: Create datasource YAML**

`server/grafana/provisioning/datasources/influxdb.yaml`:

```yaml
apiVersion: 1
datasources:
  - name: InfluxDB-OpenVPN
    uid: influxdb-openvpn
    type: influxdb
    access: proxy
    url: http://openvpn-influxdb3:8181
    basicAuth: false
    jsonData:
      version: "SQL"
      dbName: openvpn
      httpMode: POST
      insecureSkipVerify: true
    secureJsonData:
      token: "${INFLUX_TOKEN}"
    editable: false
```

- [ ] **Step 2: Create dashboard provider YAML**

`server/grafana/provisioning/dashboards/provider.yaml`:

```yaml
apiVersion: 1
providers:
  - name: openvpn-dashboards
    type: file
    disableDeletion: true
    updateIntervalSeconds: 30
    allowUiUpdates: false
    options:
      path: /etc/grafana/provisioning/dashboards
      foldersFromFilesStructure: false
```

- [ ] **Step 3: Update `server/docker-compose.yml`**

In the `openvpn-grafana` service, add to `environment`:

```yaml
      - INFLUX_TOKEN=${INFLUX_TOKEN}
```

Add to `volumes`:

```yaml
      - /opt/docker/openvpn/grafana/provisioning:/etc/grafana/provisioning:ro
```

The grafana service section becomes:

```yaml
  openvpn-grafana:
    container_name: openvpn-grafana
    image: grafana/grafana:latest
    networks:
      - frontend
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD}
      - GF_AUTH_PROXY_ENABLED=true
      - GF_AUTH_PROXY_HEADER_NAME=X-WEBAUTH-USER
      - GF_AUTH_PROXY_AUTO_SIGN_UP=true
      - GF_AUTH_PROXY_WHITELIST=172.19.0.0/16
      - GF_USERS_AUTO_ASSIGN_ORG_ROLE=Admin
      - GF_SECURITY_ALLOW_EMBEDDING=true
      - GF_SERVER_ROOT_URL=%(protocol)s://%(domain)s/grafana
      - GF_SERVER_SERVE_FROM_SUB_PATH=true
      - GF_LOG_LEVEL=warn
      - INFLUX_TOKEN=${INFLUX_TOKEN}
    volumes:
      - /opt/docker/openvpn/grafana/data:/var/lib/grafana
      - /opt/docker/openvpn/grafana/provisioning:/etc/grafana/provisioning:ro
    restart: always
    depends_on:
      - openvpn-influxdb3
```

- [ ] **Step 4: Commit**

```bash
git add server/grafana/ server/docker-compose.yml
git commit -m "feat: add Grafana provisioning infrastructure (datasource + provider)"
git push
```

---

### Task 2: Dashboard 1 — Aktive Verbindungen

**File:** `server/grafana/provisioning/dashboards/01-active-connections.json`

Panels (24-column grid):
- Row 0 (y=0, h=4): Aktive Nutzer · Traffic Heute · Sessions Heute · Sessions Diesen Monat · Sessions Letzten Monat · Längste aktive Session
- Row 1 (y=4, h=8): Verbindungen über Zeit (Time series, last 24 h)
- Row 2 (y=12, h=9): Aktive Verbindungen — Tabelle (wer ist jetzt verbunden)

- [ ] **Step 1: Create `server/grafana/provisioning/dashboards/01-active-connections.json`**

```json
{
  "annotations": {"list": []},
  "editable": false,
  "graphTooltip": 1,
  "id": null,
  "links": [
    {"title": "Traffic-Verlauf", "url": "/grafana/d/openvpn-traffic", "type": "link", "targetBlank": false},
    {"title": "Nutzer-Detail", "url": "/grafana/d/openvpn-user", "type": "link", "targetBlank": false}
  ],
  "panels": [
    {
      "id": 1, "type": "stat", "title": "Aktive Nutzer",
      "gridPos": {"h": 4, "w": 4, "x": 0, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT COUNT(DISTINCT common_name) AS value FROM openvpn_traffic WHERE time >= NOW() - INTERVAL '2 minutes'"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "background", "graphMode": "none", "textMode": "auto"},
      "fieldConfig": {"defaults": {"unit": "short", "color": {"mode": "thresholds"}, "thresholds": {"mode": "absolute", "steps": [{"color": "green", "value": null}]}}, "overrides": []}
    },
    {
      "id": 2, "type": "stat", "title": "Traffic Heute",
      "gridPos": {"h": 4, "w": 4, "x": 4, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT SUM(bytes_in + bytes_out) AS value FROM openvpn_session WHERE time >= DATE_TRUNC('day', NOW())"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "bytes"}, "overrides": []}
    },
    {
      "id": 3, "type": "stat", "title": "Sessions Heute",
      "gridPos": {"h": 4, "w": 4, "x": 8, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT COUNT(*) AS value FROM openvpn_session WHERE time >= DATE_TRUNC('day', NOW())"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "short"}, "overrides": []}
    },
    {
      "id": 4, "type": "stat", "title": "Sessions — Dieser Monat",
      "gridPos": {"h": 4, "w": 4, "x": 12, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT COUNT(*) AS value FROM openvpn_session WHERE time >= DATE_TRUNC('month', NOW())"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "short"}, "overrides": []}
    },
    {
      "id": 5, "type": "stat", "title": "Sessions — Letzter Monat",
      "gridPos": {"h": 4, "w": 4, "x": 16, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT COUNT(*) AS value FROM openvpn_session WHERE time >= DATE_TRUNC('month', NOW()) - INTERVAL '1 month' AND time < DATE_TRUNC('month', NOW())"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "short"}, "overrides": []}
    },
    {
      "id": 6, "type": "stat", "title": "Längste aktive Session",
      "gridPos": {"h": 4, "w": 4, "x": 20, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT MAX(NOW() - MIN(time)) AS value FROM openvpn_traffic WHERE time >= NOW() - INTERVAL '24 hours' GROUP BY common_name ORDER BY value DESC LIMIT 1"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "s"}, "overrides": []}
    },
    {
      "id": 7, "type": "timeseries", "title": "Verbindungen über Zeit",
      "gridPos": {"h": 8, "w": 24, "x": 0, "y": 4},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT DATE_TRUNC('minute', time) AS time, COUNT(DISTINCT common_name) AS \"Aktive Verbindungen\" FROM openvpn_traffic WHERE $__timeFilter(time) GROUP BY DATE_TRUNC('minute', time) ORDER BY time"
      }],
      "options": {"tooltip": {"mode": "single"}, "legend": {"displayMode": "list", "placement": "bottom"}},
      "fieldConfig": {"defaults": {"unit": "short", "color": {"mode": "fixed", "fixedColor": "green"}, "custom": {"lineWidth": 2, "fillOpacity": 15, "spanNulls": true}}, "overrides": []}
    },
    {
      "id": 8, "type": "table", "title": "Aktive Verbindungen",
      "gridPos": {"h": 9, "w": 24, "x": 0, "y": 12},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT common_name AS \"Nutzer\", virtual_ip AS \"VPN IP\", real_ip AS \"Real IP\", MAX(bytes_in) AS \"Bytes In\", MAX(bytes_out) AS \"Bytes Out\", MIN(time) AS \"Verbunden seit\" FROM openvpn_traffic WHERE time >= NOW() - INTERVAL '2 minutes' GROUP BY common_name, virtual_ip, real_ip ORDER BY common_name"
      }],
      "options": {"showHeader": true, "footer": {"show": false}},
      "fieldConfig": {
        "defaults": {"custom": {"align": "auto", "displayMode": "auto"}},
        "overrides": [
          {"matcher": {"id": "byName", "options": "Bytes In"}, "properties": [{"id": "unit", "value": "bytes"}]},
          {"matcher": {"id": "byName", "options": "Bytes Out"}, "properties": [{"id": "unit", "value": "bytes"}]},
          {"matcher": {"id": "byName", "options": "Verbunden seit"}, "properties": [{"id": "unit", "value": "dateTimeAsIso"}]}
        ]
      }
    }
  ],
  "refresh": "1m",
  "schemaVersion": 39,
  "tags": ["openvpn"],
  "templating": {"list": []},
  "time": {"from": "now-24h", "to": "now"},
  "timezone": "browser",
  "title": "OpenVPN — Aktive Verbindungen",
  "uid": "openvpn-live",
  "version": 1
}
```

- [ ] **Step 2: Commit**

```bash
git add server/grafana/provisioning/dashboards/01-active-connections.json
git commit -m "feat: add Grafana dashboard 1 — aktive Verbindungen"
git push
```

---

### Task 3: Dashboard 2 — Traffic-Verlauf

**File:** `server/grafana/provisioning/dashboards/02-traffic-history.json`

Panels:
- Row 0 (y=0, h=4): Gesamt Allzeit · Gesamt Zeitraum · Sessions Zeitraum · Nutzer Gesamt Allzeit
- Row 1 (y=4, h=9): Traffic pro Tag (Bar chart, Zeitraum) · Top Nutzer (Horizontal bar, Zeitraum)
- Row 2 (y=13, h=9): Monatlicher Traffic — Alle Zeit (Bar chart, kein Zeitfilter — zeigt immer die gesamte Langzeit-Historie)
- Row 3 (y=22, h=9): Session-Tabelle (Zeitraum)

Variable: `$cn` — Nutzer-Filter, "Alle" möglich

- [ ] **Step 1: Create `server/grafana/provisioning/dashboards/02-traffic-history.json`**

```json
{
  "annotations": {"list": []},
  "editable": false,
  "graphTooltip": 1,
  "id": null,
  "links": [
    {"title": "Aktive Verbindungen", "url": "/grafana/d/openvpn-live", "type": "link", "targetBlank": false},
    {"title": "Nutzer-Detail", "url": "/grafana/d/openvpn-user", "type": "link", "targetBlank": false}
  ],
  "panels": [
    {
      "id": 1, "type": "stat", "title": "Gesamt-Traffic — Allzeit",
      "description": "Gesamter übertragener Traffic seit Beginn der Aufzeichnung — kein Zeitfilter.",
      "gridPos": {"h": 4, "w": 6, "x": 0, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT SUM(bytes_in + bytes_out) AS value FROM openvpn_session"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "bytes"}, "overrides": []}
    },
    {
      "id": 2, "type": "stat", "title": "Traffic — Gewählter Zeitraum",
      "gridPos": {"h": 4, "w": 6, "x": 6, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT SUM(bytes_in + bytes_out) AS value FROM openvpn_session WHERE $__timeFilter(time)"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "bytes"}, "overrides": []}
    },
    {
      "id": 3, "type": "stat", "title": "Sessions — Gewählter Zeitraum",
      "gridPos": {"h": 4, "w": 6, "x": 12, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT COUNT(*) AS value FROM openvpn_session WHERE $__timeFilter(time)"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "short"}, "overrides": []}
    },
    {
      "id": 4, "type": "stat", "title": "Nutzer Gesamt — Allzeit",
      "description": "Distinct CNs mit mindestens einer Session.",
      "gridPos": {"h": 4, "w": 6, "x": 18, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT COUNT(DISTINCT common_name) AS value FROM openvpn_session"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "short"}, "overrides": []}
    },
    {
      "id": 5, "type": "barchart", "title": "Traffic pro Tag",
      "gridPos": {"h": 9, "w": 14, "x": 0, "y": 4},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT DATE_TRUNC('day', time) AS time, SUM(bytes_in) AS \"Bytes In\", SUM(bytes_out) AS \"Bytes Out\" FROM openvpn_session WHERE $__timeFilter(time) GROUP BY DATE_TRUNC('day', time) ORDER BY time"
      }],
      "options": {"xField": "time", "stacking": {"mode": "normal"}, "tooltip": {"mode": "multi"}, "legend": {"displayMode": "list", "placement": "bottom"}},
      "fieldConfig": {"defaults": {"unit": "bytes"}, "overrides": [
        {"matcher": {"id": "byName", "options": "Bytes In"}, "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": "blue"}}]},
        {"matcher": {"id": "byName", "options": "Bytes Out"}, "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": "orange"}}]}
      ]}
    },
    {
      "id": 6, "type": "barchart", "title": "Top Nutzer — Zeitraum",
      "gridPos": {"h": 9, "w": 10, "x": 14, "y": 4},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT common_name AS \"Nutzer\", SUM(bytes_in + bytes_out) AS \"Traffic\" FROM openvpn_session WHERE $__timeFilter(time) GROUP BY common_name ORDER BY SUM(bytes_in + bytes_out) DESC LIMIT 10"
      }],
      "options": {"xField": "Nutzer", "tooltip": {"mode": "single"}, "legend": {"displayMode": "list", "placement": "bottom"}, "orientation": "horizontal"},
      "fieldConfig": {"defaults": {"unit": "bytes", "color": {"mode": "fixed", "fixedColor": "green"}}, "overrides": []}
    },
    {
      "id": 7, "type": "barchart", "title": "Monatlicher Traffic — Gesamte Historie",
      "description": "Kein Zeitraum-Filter — zeigt immer alle Monate seit Aufzeichnungsbeginn.",
      "gridPos": {"h": 9, "w": 24, "x": 0, "y": 13},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT DATE_TRUNC('month', time) AS time, SUM(bytes_in) AS \"Bytes In\", SUM(bytes_out) AS \"Bytes Out\" FROM openvpn_session GROUP BY DATE_TRUNC('month', time) ORDER BY time"
      }],
      "options": {"xField": "time", "stacking": {"mode": "normal"}, "tooltip": {"mode": "multi"}, "legend": {"displayMode": "list", "placement": "bottom"}},
      "fieldConfig": {"defaults": {"unit": "bytes"}, "overrides": [
        {"matcher": {"id": "byName", "options": "Bytes In"}, "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": "blue"}}]},
        {"matcher": {"id": "byName", "options": "Bytes Out"}, "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": "orange"}}]}
      ]}
    },
    {
      "id": 8, "type": "table", "title": "Sessions — Zeitraum",
      "gridPos": {"h": 9, "w": 24, "x": 0, "y": 22},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT time AS \"Zeitpunkt\", common_name AS \"Nutzer\", real_ip AS \"Real IP\", virtual_ip AS \"VPN IP\", duration_s AS \"Dauer (s)\", bytes_in AS \"Bytes In\", bytes_out AS \"Bytes Out\" FROM openvpn_session WHERE $__timeFilter(time) ORDER BY time DESC LIMIT 200"
      }],
      "options": {"showHeader": true, "footer": {"show": false}},
      "fieldConfig": {
        "defaults": {"custom": {"align": "auto", "displayMode": "auto"}},
        "overrides": [
          {"matcher": {"id": "byName", "options": "Bytes In"}, "properties": [{"id": "unit", "value": "bytes"}]},
          {"matcher": {"id": "byName", "options": "Bytes Out"}, "properties": [{"id": "unit", "value": "bytes"}]},
          {"matcher": {"id": "byName", "options": "Dauer (s)"}, "properties": [{"id": "unit", "value": "s"}]},
          {"matcher": {"id": "byName", "options": "Zeitpunkt"}, "properties": [{"id": "unit", "value": "dateTimeAsIso"}]}
        ]
      }
    }
  ],
  "refresh": "5m",
  "schemaVersion": 39,
  "tags": ["openvpn"],
  "templating": {"list": []},
  "time": {"from": "now-30d", "to": "now"},
  "timezone": "browser",
  "title": "OpenVPN — Traffic-Verlauf",
  "uid": "openvpn-traffic",
  "version": 1
}
```

- [ ] **Step 2: Commit**

```bash
git add server/grafana/provisioning/dashboards/02-traffic-history.json
git commit -m "feat: add Grafana dashboard 2 — Traffic-Verlauf"
git push
```

---

### Task 4: Dashboard 3 — Nutzer-Detail

**File:** `server/grafana/provisioning/dashboards/03-user-detail.json`

Panels:
- Row 0 (y=0, h=4): Allzeit-Traffic · Allzeit-Sessions · Ø Sitzungsdauer · Erste Verbindung · Letzte Verbindung
- Row 1 (y=4, h=9): Monatlicher Traffic — Gesamte Historie für diesen Nutzer (kein Zeitfilter)
- Row 2 (y=13, h=8): Durchsatz im gewählten Zeitraum (Time series, delta aus kumulativen Zählern)
- Row 3 (y=21, h=8): Real-IP-Übersicht · Sitzungsverlauf (Tabelle)

Variable: `$cn` (Pflicht, kein "Alle" — Nutzer muss gewählt werden)

- [ ] **Step 1: Create `server/grafana/provisioning/dashboards/03-user-detail.json`**

```json
{
  "annotations": {"list": []},
  "editable": false,
  "graphTooltip": 1,
  "id": null,
  "links": [
    {"title": "Aktive Verbindungen", "url": "/grafana/d/openvpn-live", "type": "link", "targetBlank": false},
    {"title": "Traffic-Verlauf", "url": "/grafana/d/openvpn-traffic", "type": "link", "targetBlank": false}
  ],
  "panels": [
    {
      "id": 1, "type": "stat", "title": "Gesamt-Traffic — Allzeit",
      "description": "Kein Zeitraum-Filter — gesamter Traffic dieses Nutzers seit Aufzeichnungsbeginn.",
      "gridPos": {"h": 4, "w": 6, "x": 0, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT SUM(bytes_in + bytes_out) AS value FROM openvpn_session WHERE common_name = '$cn'"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "bytes"}, "overrides": []}
    },
    {
      "id": 2, "type": "stat", "title": "Sessions — Allzeit",
      "gridPos": {"h": 4, "w": 4, "x": 6, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT COUNT(*) AS value FROM openvpn_session WHERE common_name = '$cn'"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "short"}, "overrides": []}
    },
    {
      "id": 3, "type": "stat", "title": "Ø Sitzungsdauer — Allzeit",
      "gridPos": {"h": 4, "w": 4, "x": 10, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT AVG(duration_s) AS value FROM openvpn_session WHERE common_name = '$cn'"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "s"}, "overrides": []}
    },
    {
      "id": 4, "type": "stat", "title": "Erste Verbindung",
      "gridPos": {"h": 4, "w": 5, "x": 14, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT MIN(time) AS value FROM openvpn_session WHERE common_name = '$cn'"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "dateTimeAsIso"}, "overrides": []}
    },
    {
      "id": 5, "type": "stat", "title": "Letzte Verbindung",
      "gridPos": {"h": 4, "w": 5, "x": 19, "y": 0},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT MAX(time) AS value FROM openvpn_session WHERE common_name = '$cn'"
      }],
      "options": {"reduceOptions": {"calcs": ["lastNotNull"]}, "colorMode": "none", "graphMode": "none"},
      "fieldConfig": {"defaults": {"unit": "dateTimeAsIso"}, "overrides": []}
    },
    {
      "id": 6, "type": "barchart", "title": "Monatlicher Traffic — Gesamte Historie",
      "description": "Kein Zeitraum-Filter — alle Monate seit Aufzeichnungsbeginn für diesen Nutzer.",
      "gridPos": {"h": 9, "w": 24, "x": 0, "y": 4},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT DATE_TRUNC('month', time) AS time, SUM(bytes_in) AS \"Bytes In\", SUM(bytes_out) AS \"Bytes Out\" FROM openvpn_session WHERE common_name = '$cn' GROUP BY DATE_TRUNC('month', time) ORDER BY time"
      }],
      "options": {"xField": "time", "stacking": {"mode": "normal"}, "tooltip": {"mode": "multi"}, "legend": {"displayMode": "list", "placement": "bottom"}},
      "fieldConfig": {"defaults": {"unit": "bytes"}, "overrides": [
        {"matcher": {"id": "byName", "options": "Bytes In"}, "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": "blue"}}]},
        {"matcher": {"id": "byName", "options": "Bytes Out"}, "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": "orange"}}]}
      ]}
    },
    {
      "id": 7, "type": "timeseries", "title": "Durchsatz im Zeitraum",
      "description": "Delta aus kumulativen bytes_in/bytes_out. Negative Deltas (Reconnect) werden ausgeblendet.",
      "gridPos": {"h": 8, "w": 24, "x": 0, "y": 13},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "WITH ordered AS (SELECT time, bytes_in, bytes_out, LAG(bytes_in) OVER (ORDER BY time) AS prev_in, LAG(bytes_out) OVER (ORDER BY time) AS prev_out FROM openvpn_traffic WHERE common_name = '$cn' AND $__timeFilter(time)) SELECT time, CASE WHEN bytes_in >= prev_in THEN bytes_in - prev_in ELSE 0 END AS \"Bytes In\", CASE WHEN bytes_out >= prev_out THEN bytes_out - prev_out ELSE 0 END AS \"Bytes Out\" FROM ordered WHERE prev_in IS NOT NULL ORDER BY time"
      }],
      "options": {"tooltip": {"mode": "multi", "sort": "desc"}, "legend": {"displayMode": "list", "placement": "bottom"}},
      "fieldConfig": {"defaults": {"unit": "bytes", "custom": {"lineWidth": 2, "fillOpacity": 10, "spanNulls": false}}, "overrides": [
        {"matcher": {"id": "byName", "options": "Bytes In"}, "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": "blue"}}]},
        {"matcher": {"id": "byName", "options": "Bytes Out"}, "properties": [{"id": "color", "value": {"mode": "fixed", "fixedColor": "orange"}}]}
      ]}
    },
    {
      "id": 8, "type": "table", "title": "Verwendete Real-IPs — Allzeit",
      "gridPos": {"h": 8, "w": 10, "x": 0, "y": 21},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT real_ip AS \"Real IP\", COUNT(*) AS \"Sessions\", SUM(duration_s) AS \"Gesamtdauer (s)\", MAX(time) AS \"Zuletzt gesehen\" FROM openvpn_session WHERE common_name = '$cn' GROUP BY real_ip ORDER BY COUNT(*) DESC"
      }],
      "options": {"showHeader": true, "footer": {"show": false}},
      "fieldConfig": {
        "defaults": {"custom": {"align": "auto", "displayMode": "auto"}},
        "overrides": [
          {"matcher": {"id": "byName", "options": "Gesamtdauer (s)"}, "properties": [{"id": "unit", "value": "s"}]},
          {"matcher": {"id": "byName", "options": "Zuletzt gesehen"}, "properties": [{"id": "unit", "value": "dateTimeAsIso"}]}
        ]
      }
    },
    {
      "id": 9, "type": "table", "title": "Sitzungsverlauf — Zeitraum",
      "gridPos": {"h": 8, "w": 14, "x": 10, "y": 21},
      "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
      "targets": [{"datasource": {"type": "influxdb", "uid": "influxdb-openvpn"}, "rawQuery": true, "refId": "A",
        "query": "SELECT time AS \"Disconnect\", real_ip AS \"Real IP\", virtual_ip AS \"VPN IP\", duration_s AS \"Dauer (s)\", bytes_in AS \"Bytes In\", bytes_out AS \"Bytes Out\" FROM openvpn_session WHERE common_name = '$cn' AND $__timeFilter(time) ORDER BY time DESC LIMIT 100"
      }],
      "options": {"showHeader": true, "footer": {"show": false}},
      "fieldConfig": {
        "defaults": {"custom": {"align": "auto", "displayMode": "auto"}},
        "overrides": [
          {"matcher": {"id": "byName", "options": "Bytes In"}, "properties": [{"id": "unit", "value": "bytes"}]},
          {"matcher": {"id": "byName", "options": "Bytes Out"}, "properties": [{"id": "unit", "value": "bytes"}]},
          {"matcher": {"id": "byName", "options": "Dauer (s)"}, "properties": [{"id": "unit", "value": "s"}]},
          {"matcher": {"id": "byName", "options": "Disconnect"}, "properties": [{"id": "unit", "value": "dateTimeAsIso"}]}
        ]
      }
    }
  ],
  "refresh": "5m",
  "schemaVersion": 39,
  "tags": ["openvpn"],
  "templating": {
    "list": [
      {
        "name": "cn",
        "label": "Nutzer",
        "type": "query",
        "datasource": {"type": "influxdb", "uid": "influxdb-openvpn"},
        "definition": "SELECT DISTINCT common_name FROM openvpn_session ORDER BY common_name",
        "query": {"query": "SELECT DISTINCT common_name FROM openvpn_session ORDER BY common_name", "rawQuery": true},
        "includeAll": false,
        "multi": false,
        "refresh": 2,
        "sort": 1,
        "current": {}
      }
    ]
  },
  "time": {"from": "now-30d", "to": "now"},
  "timezone": "browser",
  "title": "OpenVPN — Nutzer-Detail",
  "uid": "openvpn-user",
  "version": 1
}
```

- [ ] **Step 2: Commit**

```bash
git add server/grafana/provisioning/dashboards/03-user-detail.json
git commit -m "feat: add Grafana dashboard 3 — Nutzer-Detail"
git push
```

---

### Task 5: Deploy + Verify

- [ ] **Step 1: Provisioning-Verzeichnis auf den Server kopieren**

```bash
# Auf dem Server ausführen:
mkdir -p /opt/docker/openvpn/grafana/provisioning/datasources
mkdir -p /opt/docker/openvpn/grafana/provisioning/dashboards

# Dann die Dateien übertragen (z.B. via scp oder git pull auf dem Server):
cp server/grafana/provisioning/datasources/influxdb.yaml \
   /opt/docker/openvpn/grafana/provisioning/datasources/
cp server/grafana/provisioning/dashboards/provider.yaml \
   /opt/docker/openvpn/grafana/provisioning/dashboards/
cp server/grafana/provisioning/dashboards/*.json \
   /opt/docker/openvpn/grafana/provisioning/dashboards/
```

- [ ] **Step 2: Grafana-Container neu starten**

```bash
docker compose restart openvpn-grafana
```

Provisioning wird beim Start geladen. Container-Log prüfen:

```bash
docker logs openvpn-grafana 2>&1 | grep -E "provisioning|datasource|dashboard|error"
```

Erwartete Ausgabe enthält (kein Error):
```
msg="starting to provision datasources"
msg="inserting datasource from configuration"  name=InfluxDB-OpenVPN
msg="starting to provision dashboards"
msg="inserting dashboard" name="OpenVPN — Aktive Verbindungen"
msg="inserting dashboard" name="OpenVPN — Traffic-Verlauf"
msg="inserting dashboard" name="OpenVPN — Nutzer-Detail"
```

- [ ] **Step 3: Manueller Test-Checkliste**

1. **Datasource**: Grafana → Connections → Data Sources → InfluxDB-OpenVPN → "Test" klicken → grüner Haken.
2. **Dashboard 1**: Öffnen → Stat "Aktive Nutzer" zeigt 0 oder > 0 (je nach aktiver Verbindung) → kein "No data" auf den Stats.
3. **Dashboard 1, Sessions-Monat**: Bei frischem System zeigt "Dieser Monat" die gleiche Zahl wie Sessions heute — korrekt.
4. **Dashboard 2**: Zeitraum auf "Last 90 days" setzen → "Monatlicher Traffic" zeigt immer alle Monate (nicht nur 90d) — das ist das Langzeit-Panel ohne `$__timeFilter`.
5. **Dashboard 3**: CN-Dropdown befüllt sich aus der DB → nach Auswahl laden alle Panels → "Allzeit"-Stats ignorieren den Zeitraum-Picker.
6. **Durchsatz-Panel (Dashboard 3)**: Zeitraum auf eine aktive Session oder vergangene Session setzen → Time series zeigt Bytes In / Out als Delta-Werte ohne negative Spikes.

- [ ] **Step 4: Falls Datasource-Typ nicht passt**

Wenn der Standard-InfluxDB-Plugin kein SQL unterstützt (Grafana < 10.3 oder Plugin-Version ohne SQL-Modus), Plugin installieren:

```bash
docker exec openvpn-grafana grafana-cli plugins install grafana-flightsql-datasource
docker restart openvpn-grafana
```

Dann in `influxdb.yaml` den `type` ändern:
```yaml
type: grafana-flightsql-datasource
jsonData:
  host: openvpn-influxdb3:31337   # Flight SQL Port (Standard: 31337 für InfluxDB v3)
  database: openvpn
  insecure: true
```

---

## Langzeit-Design-Entscheidungen

| Panel | Zeitfilter | Warum |
|---|---|---|
| Monatlicher Traffic (Dashboard 2 + 3) | Kein `$__timeFilter` | Zeigt immer die vollständige Monatskurve — nicht auf den gewählten Zeitraum beschränkt |
| Allzeit-Stats (alle Dashboards) | Kein Filter | Absolute Gesamtwerte sind immer nützlich, unabhängig vom Zoom |
| Sitzungsverlauf (Dashboard 3) | `$__timeFilter` | Zu viele Einträge ohne Einschränkung — explizit durchblättern via Zeitraum |
| Session-Tabelle (Dashboard 2) | `$__timeFilter` + LIMIT 200 | Schutz vor zu großen Ergebnismengen |
| Durchsatz-Timeseries (Dashboard 3) | `$__timeFilter` | Scrape-Punkte bei 60s-Intervall: 30 Tage = 43.200 Punkte — muss eingeschränkt werden |
