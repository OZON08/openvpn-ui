# Roadmap

Priority-ordered, no dates. Items closer to the top are more likely to land
sooner. Nothing here is a commitment — open an issue or PR if you want to
accelerate something or propose an alternative.

## Near term (v0.9.x)

- [ ] **Verify InfluxDB v3 export end-to-end** against a live InfluxDB 3 Core
      instance. Currently flagged untested — see the warning in
      [README.md](README.md) and [docs/docker-compose.influx.yml](docs/docker-compose.influx.yml).
      Covers the full path: scraper → `InfluxWriter` buffer → HTTP write →
      query of `traffic` / `session` measurements.
- [ ] **Grafana dashboard JSON** for the `traffic` and `session` measurements
      (per-user bandwidth, connected-clients gauge, session history panel).
      Shipped under `docs/grafana/` so users can import and tweak.
- [ ] **Light/dark toggle polish + re-enable.** The switch is commented out
      in [views/layout/base.html](views/layout/base.html); the underlying
      boot script already handles both modes. Needs styling review in light
      mode before the toggle comes back.

## Toward v1.0 (stability)

- [ ] **Test coverage for `lib/monitor` and the cert helper scripts** —
      integration tests in CI against a real `easyrsa` binary and a
      fixture `openvpn-status.log`. The cert-action bug that shipped in
      v0.9.7 would have been caught by even a smoke test.
- [ ] **OpenAPI spec for `/api/v1/monitor/*`** — schema + examples for the
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
