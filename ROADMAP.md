# Roadmap

Priority-ordered, no dates. Items closer to the top are more likely to land
sooner. Nothing here is a commitment — open an issue or PR if you want to
accelerate something or propose an alternative.

## Released (v0.9.x)

- [x] **Verify InfluxDB v3 export end-to-end** against a live InfluxDB 3 Core
      instance. Verified: scraper → `InfluxWriter` buffer → HTTP write →
      `openvpn_traffic` / `openvpn_session` measurements confirmed in InfluxDB.
- [x] **Grafana integration** — reverse proxy behind openvpn-ui auth, embedded
      in the sidebar via iframe. InfluxDB v3 data source verified against
      `openvpn_traffic` and `openvpn_session` measurements. Setup documented
      in README. Three provisioned dashboards ship with the repo:
      *Aktive Verbindungen* (live, 1 min refresh), *Traffic-Verlauf* (30d
      history, monthly long-term chart), *Nutzer-Detail* (per-CN drill-down
      with LAG-based throughput chart). Auto-loaded via
      `grafana/provisioning/` at container start.
- [x] **Per-user certificate and log scoping** — non-admin users see only
      their assigned certificates and matching log lines. Cert names are
      mapped explicitly via the admin Profile → Certificate Assignments tab
      (`user_certificates` SQLite table, auto-migrated at startup). Admins
      retain full access to all certs and actions. Migration: use the
      "Seed from Login Names" button to pre-fill assignments from existing
      `user.Login == cert.CN` pairs.

- [x] **Hide server stat boxes on home page for non-admin users** — the four info boxes
      (Connected clients, Bytes in/out, Load Average, OS Uptime, Server Time) and the
      Memory usage section are now wrapped in an admin-only guard. Non-admin users see
      only the Connected clients table (already scoped to their assigned certs).

- [x] **Light/dark toggle** — Three-state theme switcher in the top navbar:
      Light, Auto (follows OS `prefers-color-scheme`, default), and Dark.
      Preference is persisted in `localStorage`. CSS variables split into
      `:root` (light defaults) and `body.dark-mode` (dark overrides) in
      `v097-custom.css`; OS changes in Auto mode are applied live.

## Near term (v1.0.0)

- [x] **Test coverage for `lib/monitor` and `lib/certificates`** —
      table-driven unit tests for the OpenVPN status-log parser
      (`ParseStatusLog`, `parseConnectedSince`, `splitHostPort`, `FormatAddr`)
      with fixture files in `lib/monitor/testdata/`, and unit tests for
      the certificate layer (`ReadCerts` happy path, `parseDetails` field
      coverage, `validateCertInputs`, `SafeNameRegex`, `trim`) with a
      fixture in `lib/testdata/`. 28 tests total; no external dependencies.
- [x] **OpenAPI spec for `/api/v1/monitor/*`** — schema + examples for the
      sessions / traffic / retention / influx endpoints, rendered under
      `/api/docs` (Swagger UI or similar).
- [ ] **Audit log for admin actions** — login, cert create/revoke/renew,
      settings changes, InfluxDB reconfigure. Persisted in SQLite with a
      retention window; surfaced on a new tab (or under Monitor).

## Post-1.0 (nice to have)

- [ ] **Prometheus `/metrics` endpoint** alongside the Influx writer, so
      users who prefer Prometheus/Grafana don't have to deploy InfluxDB.
- [ ] **Alert thresholds per common-name** — bandwidth, session duration,
      unusual real-IP change. Webhook / email notification.
- [ ] **IPv6-first status parsing.** The current parser assumes IPv4 for
      `real_ip` / `virtual_ip`; verify behaviour on IPv6-only tunnels.
- [ ] **OpenVPN server config wizard** — bundled templates for common
      topologies (road-warrior, site-to-site, bridge) so new installs
      don't start from a blank `server.conf`.

---

## Contributing

PRs welcome on any of the above — open an issue first for the larger items
(tests, OpenAPI, audit log) so we can sketch the shape together before
you invest time.
