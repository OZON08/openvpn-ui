# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

---

## [0.9.7.2] - 2026-05-13

### Security

- **Non-root container process** (defense-in-depth against CVE-2026-31431 and
  similar local privilege escalation vectors). A dedicated `openvpn-ui` user
  (UID/GID 1000) is now created in the image. `start.sh` performs root-level
  provisioning (directory creation, volume permission setup) and then drops
  to `openvpn-ui` via `su-exec` before executing the web process. All shell
  scripts invoked by the Go binary (cert create/revoke/renew/burn/remove) run
  as `openvpn-ui` as well; `start.sh` pre-sets group write permissions on the
  mounted volumes (`/etc/openvpn`, `/usr/share/easy-rsa`) so the scripts
  continue to function without root.

### Added

- **Playwright screenshot automation** under `docs/screenshots/` — a small
  Node script (`capture.mjs`) plus `package.json`, `.env.example`, and a
  README. Logs into a running instance, walks every documented view
  (including the four Monitor tabs), and writes refreshed full-page PNGs
  into `docs/images/` using the same filenames the top-level README
  already references. Run with `npm install && npm run capture` after
  setting `UI_URL` / `UI_USER` / `UI_PASS`.

### Fixed

- **Missing user icon in the top navbar.** `<i class="fa fa-user-circle">`
  did not render because `font-awesome.min.4.5.0.css` is loaded after FA
  5.15.3 and overrides `.fa` → `font-family: FontAwesome`, while the
  codepoint emitted by FA 5 (`\f2bd`) does not exist in the FA 4 font
  (FA 4 maps user-circle to `\f2be`). Switched the navbar glyph to
  `fas fa-user-circle`, which uses the FA-5-only `.fas` class that isn't
  overridden by the FA 4 stylesheet. Other FA-5-only classes across the
  views (e.g. `fa fa-user-cog`) are likely affected by the same shadowing
  and will be migrated incrementally.

---

## [0.9.7.1] - 2026-04-22

### Fixed

- **Certificate actions failed with `exit status 1`** in the released
  container. The `easyrsa` helper scripts
  (`genclient.sh`, `revoke.sh`, `rmcert.sh`, `renew.sh`, `remove.sh`,
  `generate_ca_and_server_certs.sh`) read `EasyRsaPath` and `OpenVpnPath`
  through a relative path (`../openvpn-ui/conf/app.conf`) that only
  resolved when the working directory was `/opt/openvpn` — but Go invokes
  them with `cmd.Dir = /etc/openvpn`, so the grep came up empty and
  `sed` blew up on a zero-length path. `lib/certificates.go` now exports
  `EASY_RSA` and `OPENVPN_DIR` into the script environment via a new
  `buildOpenVPNEnv()` helper, and each script falls back to
  `/opt/openvpn-ui/conf/app.conf` (absolute) only when the env vars are
  absent (i.e. for manual CLI runs).

- **`Dockerfile.ci` COPY ordering** — `build/assets/app.conf` was copied
  *before* the builder stage's `/app/conf` tree, so the dev-time
  `conf/app.conf` (relative `OpenVpnPath="./openvpn"`) silently overwrote
  the container variant that uses absolute paths. Moved the `build/assets`
  COPY to after the builder COPY.

### Changed

- Top-bar **light/dark toggle hidden** — the switch in
  `views/layout/base.html` is commented out; dark mode is applied via the
  existing boot script so the forced default stays in effect. Remove the
  `{{/* … */}}` wrapper to restore the toggle.

---

## [0.9.7] - 2026-04-21

### Added

- **Long-term user monitoring** — new `Monitor` page and `/api/v1/monitor/*`
  endpoints track per-user transfer volume, connect/disconnect sessions, and
  real/virtual IP addresses.
  - A background scraper polls `openvpn-status.log` (default every 60s) and
    writes `TrafficSample`, `VpnSession` rows into SQLite.
  - A retention GC aggregates 1-minute samples into `TrafficHourly`
    (default 30-day cutover) and hourly rows into `TrafficDaily`
    (default 365-day cutover), then prunes the source table. Daily rows
    are kept indefinitely.
  - Optional **InfluxDB v3** backend: when `InfluxEnabled=true`, every
    sample and closed session is also pushed to InfluxDB (async buffer,
    best-effort — SQLite stays the source of truth).
  - Optional **OpenVPN client-disconnect webhook**
    (`build/assets/client-disconnect.sh`) closes sessions with authoritative
    byte counts; authenticated with a shared `MonitoringHookToken`.
  - All settings can be overridden per-container via env vars, e.g.
    `OPENVPN_UI_MONITORING_ENABLED`, `OPENVPN_UI_INFLUX_URL`,
    `OPENVPN_UI_INFLUX_TOKEN`, `OPENVPN_UI_MONITORING_HOOK_TOKEN`.

- **Monitor page with four live tabs** — Sessions, Users, Retention,
  InfluxDB. The Sessions tab lists active clients plus the last 50
  sessions. Users aggregates per-common-name totals (session count,
  cumulative bytes, last-seen) via a single SQL `GROUP BY` over
  `vpn_session` and exposes a per-user history drawer backed by
  `/api/v1/monitor/traffic`. Retention shows row counts for
  `traffic_sample`/`traffic_hourly`/`traffic_daily` with the active
  retention windows. InfluxDB shows live writer stats (buffered points,
  24 h flushed/errors) alongside the connection form.

- **Admin-editable InfluxDB settings** — the Monitor → InfluxDB tab now
  persists `InfluxURL` / `InfluxDatabase` / `InfluxToken` into SQLite
  (new singleton `InfluxSettings` model, Id=1), layered on top of the
  `app.conf` / `OPENVPN_UI_*` defaults. Saving from the UI hot-swaps
  the live writer via `InfluxWriter.Reconfigure()` — no restart needed.
  An empty token field leaves the existing secret untouched. Only admins
  can submit the form (XSRF-protected).

- **Rolling 24 h writer stats** — `InfluxWriter` now tracks flushed and
  failed point counts in a 24-slot ring buffer (bucket = `hour-of-epoch
  % 24`) plus an atomic depth counter, surfaced in the InfluxDB tab.

- **AdminLTE sidebar layout with dark-mode default** — the top navbar
  was replaced by a collapsible left sidebar (`views/common/sidebar.html`)
  with per-section icons and the Monitor entry pinned near the top.
  Dark mode is on by default (`body.dark-mode` applied from an inline
  boot script, persisted in `localStorage`). A toggle lives in the
  topbar. New stylesheet `static/css/v097-custom.css` layers the refreshed
  palette and tightens card/table spacing.

### Changed

- README shortened — the verbose forked-notice (security/bug/build bullets)
  collapsed into a one-paragraph summary pointing to this changelog.

- Login screen rewritten as a standalone page that renders outside the
  AdminLTE chrome, matching the new dark-mode palette.

### Removed

- `views/common/header-top-menu.html` — dead after the sidebar redesign.

---

## [0.9.6.1] - 2026-04-14

### Fixed

- **CRLF line endings in container scripts** — `start.sh`, `restart.sh`, and
  `easyrsa-tools.lib` had Windows-style CRLF line endings, causing
  `/bin/sh: /opt/start.sh: not found` on container start. Converted to LF.
  Added `.gitattributes` to enforce LF for all shell scripts and config files.

---

## [0.9.6] - 2026-04-14

### Security

- **Command Injection (Critical)** — All calls to EasyRSA and OpenVPN shell scripts in
  `lib/certificates.go` and `lib/dangerzone.go` were rewritten to pass arguments as
  separate `exec.Command` parameters instead of interpolating user input into a
  `bash -c` string via `fmt.Sprintf`. Environment variables are now set through
  `cmd.Env`, not through shell `export` statements. Static IP config files are written
  directly with `os.WriteFile` instead of a shell `echo` redirect. Input is validated
  against allowlist regexes (`^[a-zA-Z0-9._-]+$` for names, `^[a-zA-Z0-9 .,_-]+$`
  for text fields, `net.ParseIP` for IP addresses) before any command is executed.

- **Path Traversal in certificate download (Critical)** — `Download()` in
  `controllers/certificates.go` did not validate the `:key` route parameter
  before using it to construct file paths and the `Content-Disposition` header.
  Added `lib.SafeNameRegex` check; requests with invalid names are rejected
  with HTTP 400.

- **Path Traversal in image display (Critical)** — `DisplayImage()` in
  `controllers/certificates.go` now strips all path components from the image
  name with `filepath.Base()`, validates the result against an allowlist regex,
  and additionally verifies that the resolved path stays within the permitted
  `clients/` directory. Requests with directory traversal sequences are rejected
  with HTTP 403.

- **CSRF via static OAuth state (Critical)** — The hardcoded `oauthStateString =
  "random"` was removed. `GoogleLogin()` now generates a 16-byte cryptographically
  random state with `crypto/rand`, stores it in the server-side session, and passes it
  to the OAuth provider. `GoogleCallback()` validates the returned state against the
  session value and deletes it immediately after use, preventing replay.

- **OAuth error disclosure (High)** — `GoogleCallback()` in
  `controllers/login.go` returned raw `err.Error()` strings (containing
  internal network addresses and library details) directly to the browser.
  Errors are now logged server-side only; users receive a generic
  "Authentication failed" message.

- **LDAP Injection (High)** — `authenticateLdap()` in `lib/auth.go` now validates the
  login parameter against a strict allowlist regex before it is used in the DN bind
  string. The login value is additionally escaped with `ldap.EscapeFilter()` before
  being formatted into the bind DN.

- **ORM debug logging in production (High)** — `orm.Debug` in `models/models.go` is
  now only enabled when the environment variable `APP_ENV=development` is set.
  Previously all SQL queries (including password hashes) were written to the log in
  every environment.

- **Application running in development mode (High)** — `RunMode` in `conf/app.conf`
  changed from `dev` to `prod`. Development mode disables several Beego security
  hardening measures and exposes detailed error pages.

- **Race condition on global configuration (Medium)** — `state.GlobalCfg` was
  written in `controllers/settings.go` without synchronisation. Added
  `sync.RWMutex` in `state/state.go`; the write in `settings.go` is now
  protected by `Lock()/Unlock()`.

- **World-readable client configuration files (Medium)** — `SaveToFile()` in
  `controllers/certificates.go` now writes generated `.ovpn` files with permission
  `0600` instead of `0644`. These files contain private keys and must not be readable
  by other system users.

- **Weak password policy (Medium)** — Minimum password length increased from 6 to 12
  characters in `models/user.go`. The database column size was also widened from 32 to
  128 characters to accommodate stronger hashes.

- **World-readable static IP config (Low)** — `os.WriteFile` in
  `lib/certificates.go` wrote static client config files with mode `0644`.
  Changed to `0640`.

### Fixed

- **Unchecked database read in `GetLogin()`** — `controllers/base.go` called
  `u.Read("Id")` without checking the returned error. If the user record had
  been deleted since the session was created, a partially-initialised `User`
  struct was returned and could cause nil-pointer panics. The function now
  returns `nil` on any read error.

- **Nil pointer panic on log file open failure** — `controllers/logs.go` logged the
  error from `os.Open` but did not return, causing `bufio.NewScanner` to receive a nil
  file handle and panic. A `return` statement was added after the error log.

- **Variable shadowing in log viewer** — `var logs []string` in `controllers/logs.go`
  shadowed the imported `logs` package, making the logging package inaccessible for the
  remainder of the function. The local variable was renamed to `logLines`.

- **Always-true condition in certificate parser** — `strings.Contains(line, "")` in
  `lib/certificates.go` (`parseDetails`) is unconditionally true and was masking the
  intended empty-line check. Replaced with `if line == "" { continue }`.

- **Index out of bounds in certificate parser** — `parseDetails` in
  `lib/certificates.go` accessed `fields[1]` without verifying that the `=`-split
  produced at least two tokens, which would panic on malformed input lines. A
  `len(fields) < 2` guard was added.

- **Nil pointer panic in `GetLogin()`** — `controllers/base.go` performed an
  unconditional type assertion `c.GetSession("userinfo").(int64)`, which panics when
  the session key is absent. Replaced with the comma-ok form; `GetLogin()` now returns
  `nil` when no session is present.

- **Index out of bounds in `SetParams()`** — `controllers/base.go` accessed `v[0]` for
  every form value without checking whether the slice was non-empty. A `len(v) > 0`
  guard was added.

- **Redundant assignments in `EditUser()`** — `controllers/profile.go` assigned
  `user.Name` and `user.Email` unconditionally and then immediately re-assigned them
  inside `if username != ""` / `if email != ""` blocks. The unconditional assignments
  were removed.

- **Index out of bounds in OAuth email validation** — `controllers/login.go` called
  `strings.Split(email, "@")[1]` without checking the slice length, which panics for
  any email address that contains no `@`. The split result is now validated before use.
  The domain comparison was also changed to `strings.EqualFold` to be case-insensitive.

- **Compile error** — `strconv.Quote` calls remained in a `logs.Info` statement in
  `controllers/certificates.go` after the `strconv` import was removed. The calls were
  unnecessary and have been dropped.

### Build

- **Go updated to 1.25.0** — `go.mod` directive and all builder images
  (`Dockerfile-beego`, `build.sh`) updated from 1.23.4 to 1.25.0. Transitive
  dependencies updated to versions requiring Go 1.25.

- **Dependencies updated** — All direct dependencies bumped to latest releases:
  `beego/beego/v2` v2.3.10, `go-ldap/ldap/v3` v3.4.13,
  `mattn/go-sqlite3` v1.14.42, `cloudfoundry/gosigar` v1.3.117,
  `golang.org/x/oauth2` v0.36.0, `google.golang.org/api` v0.275.0.
  Indirect dependencies updated accordingly.

- **`beego/beego/v2` v2.3.10 breaking change** — `FlashData.Error()`,
  `FlashData.Warning()`, and `FlashData.Success()` became printf-style
  functions. All 42 call sites across 8 controller files updated to pass
  `"%s"` as the format string.

- **`OZON08/openvpn-server-config` updated to v0.4.0** — vendor directory
  and `go.mod` updated to the new release of the forked library.

- **`OZON08/qrencode` pinned to v0.2.0** — `Dockerfile-beego` and
  `standalone-install.sh` updated from `--branch main` to `--branch v0.2.0`.

- **Non-reproducible bee installation** — `Dockerfile-beego` installed
  `github.com/beego/bee/v2@develop` (HEAD of the develop branch), which could silently
  pick up incompatible changes between builds. Pinned to `@v2.3.0`.

- **Non-reproducible qrencode clone** — `Dockerfile-beego` cloned
  `github.com/OZON08/qrencode` without specifying a branch or commit. Changed to
  `--depth 1 --branch main` so the build is deterministic.

- **Dockerfiles left corrupted on build failure** — `build.sh` patched
  `Dockerfile` and `Dockerfile-beego` in-place with `sed -i` and never
  restored them. A failed build left the architecture baked into the files,
  causing all subsequent runs to fail silently (the placeholder `FROM DEFINE-YOUR-ARCH`
  was no longer present to be replaced). Added a `trap restore_dockerfiles EXIT` that
  restores the placeholder on both success and failure.

- **Inconsistent Go version on armv6** — `build.sh` used `golang:1.21-bookworm` for
  armv6 while all other platforms used `1.23.4`. Aligned to `1.25.0`.

- **Missing architecture prefix for arm64/aarch64** — `build.sh` specified
  `golang:1.23.4-bookworm` (amd64 image) for arm64 and aarch64 hosts instead of
  `arm64v8/golang:1.23.4-bookworm`. Fixed for both `arm64` and `aarch64` cases.

- **Missing `.dockerignore`** — The Docker build context included
  `vendor/` (~40 MB), `build/`, `.git/`, and other directories that are not
  needed in the final image. Added `.dockerignore` to exclude them and speed up
  `docker build`.

- **Placeholder OAuth credentials in image** — `build/assets/app.conf` contained
  `googleClientID = your-google-clientid` and related placeholder values that were
  baked into every built image. Replaced with empty values; a comment explains that
  these are read from environment variables at runtime.

- **Missing LocalIP in revoke index entry** — `build/assets/revoke.sh`
  appended `LocalIP=${CERT_IP}` to the `index.txt` entry on revocation, but
  `CERT_IP` is never set by the UI for revoke operations, resulting in `LocalIP=`
  (empty) in the database. The `LocalIP` field was removed from the revoke path;
  IP assignment is not relevant after a certificate is revoked.

### Changed

- **Repository rebranded to `OZON08/openvpn-ui`** — Go module path updated from
  `github.com/d3vilh/openvpn-ui` to `github.com/OZON08/openvpn-ui`. All internal
  import paths, Docker image references, build volume mounts, and documentation
  updated accordingly. The Go library dependency `d3vilh/openvpn-server-config`
  remains at its upstream path until that fork is published with an updated
  module name.

- `strconv.Quote` wrapping removed from the `City`, `Org`, and `OrgUnit` parameters
  passed to `lib.CreateCertificate()`. Shell-level quoting is no longer needed because
  values are passed as environment variables rather than embedded in a shell command
  string.

- `safeTextRegex` extended to allow `@` and `+` characters so that email addresses
  are accepted as valid input in certificate fields.

---

## [0.9.5.6] - 2024-12-29

### Fixed

- Updated `openssl-easyrsa.cnf` location
- Removed deprecated EasyRSA `req-cn` subject field following latest EasyRSA team
  recommendations
- Bugfix for issue [#114](https://github.com/OZON08/openvpn-ui/issues/114)
- More output during `gen-crl` process
- Password change dialog clarification
- Icon and UI updates

### Changed

- Exclamation symbols deprecated in configuration
- Workaround for renewed certificate process

---

## [0.9.5.5] - 2024-12-26

### Added

- Google OAuth 2.0 login support (client ID, secret and redirect URL configured via
  environment variables `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`,
  `GOOGLE_REDIRECT_URL`, `ALLOWED_DOMAINS`)
- New login screen UI

### Changed

- Upgraded to Go 1.23.4 and Beego 2.3.4
