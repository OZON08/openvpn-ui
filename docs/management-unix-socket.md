# Management interface over a Unix socket

Status: 2026-10-31. The change is committed in this repo. The complete stack is not tested yet, see [Open](#open).

## Goal

The UI controls OpenVPN through `/etc/openvpn/run/mi.sock` instead of TCP port 2080. The interface has no password, so anyone who reached the port could read client data, disconnect clients or stop the server.

The matching server change is in the `openvpn-server` repo, version v0.6.4, in `docs/management-unix-socket.md`.

## Changes in this repo

| File | Change |
|---|---|
| `conf/app.conf`, `build/assets/app.conf` | Management address and network set to the socket |
| `conf/openvpn-server-config.tpl` | `management` line uses the socket, it was fixed to `0.0.0.0 2080` before |
| `models/models.go`, `models/management_test.go` | Stored settings with the old default `openvpn:2080` move to the socket at startup. Values changed by an administrator are kept |
| `views/settings.html` | Placeholders show the socket |
| `docs/docker-compose.yml`, `docs/docker-compose.influx.yml` | New `./run:/etc/openvpn/run` volume, healthcheck checks socket and process, port entry removed |
| `go.mod`, `go.sum`, `vendor/` | `openvpn-server-config` v0.5.0, Go 1.26.10 and security updates for `x/net`, `x/text`, `grpc`, `go-ntlmssp` and `klauspost/compress` |
| `build/build.sh`, `build/Dockerfile-beego`, `Dockerfile.ci`, `build/standalone-install.sh` | Go 1.26.10. `build/build.sh` rejects armv5 and armv6 |
| `CHANGELOG.md` | Entry for this change |
| `README.md`, `docs/superpowers/plans/2026-06-20-statuslog-tests.md` | Real name removed |

These files are not part of this change and must stay out of its commit:

- `static/css/v097-custom.css`
- the untracked plans in `docs/superpowers/plans/`: `2026-06-14-grafana-dashboards.md`, `2026-06-20-auto-theme.md`, `2026-06-20-light-dark-toggle.md`

## How access is restricted

The socket has mode `0777`, so its own permissions protect nothing. Access is limited by the `run` directory, which the server container sets to `root:root` with mode `0700` at startup. Only root can therefore reach the socket. `build/assets/start.sh` starts the UI as user `openvpn-ui`, so **the UI is not expected to reach the socket yet**. See [Open](#open). In rootless Docker, container root is the user who runs Docker.

## Rollout

1. Update the `openvpn` and `openvpn-ui` images together. The new UI only talks to the socket.
2. Add `./run:/etc/openvpn/run` to the `openvpn` service. Without this volume an existing config stays on TCP.
3. Use the new healthcheck from the compose files. The old one tests port 2080 and would fail.

## Behaviour changes from `openvpn-server-config` v0.5.0

- **EasyRSA values:** only letters, digits, spaces and `. , _ @ & + ( ) : -` are allowed. Saving fails for stored values with other characters, for example an apostrophe.
- **OpenVPN configs:** line breaks are rejected in single-value fields. Key fields must not contain `<`.
- **File modes:** `SaveToFile` sets existing files to `0600`. The file is updated in place, so the single-file mount of `server.conf` keeps working.
- **`SIGHUP`:** OpenVPN exits on this signal when `server.conf` has mode `0600`, because it runs as `nobody` and cannot read the file again. With `restart: always` the container restarts. The UI buttons use `SIGUSR1`, but the signal API also accepts other signals.

## Tested

- Tests with `-race` and the build pass for `models`, `controllers` and `lib`.
- `govulncheck ./...` reports 0 vulnerabilities in called code. It still lists GO-2026-5932 in `x/crypto/openpgp`, which has no fix. That package is not in `vendor/`.
- The UI templates render with v0.5.0 using default values.
- The socket was tested with OpenVPN 2.6 in Docker, a client built from the library and the start block of `docker-entrypoint.sh`:
  - Root in another container connects. UID 1000 and UID 65534 get `permission denied`.
  - `SIGUSR1` and a container restart keep the socket working.
  - An old `server.conf` is migrated and keeps its inode.
  - The healthcheck passes while OpenVPN runs and fails without the process.

## Open

1. Build both images and test the complete stack: UI status over the socket, saving `server.conf` with the single-file mount, migrating an old database.
2. Give the UI access to the socket. `build/assets/start.sh` runs the web process as `openvpn-ui` through `su-exec`, but `run` is `root:root` with mode `0700`. The same script also runs `chgrp -R` and `chmod -R g+w` on `/etc/openvpn`, which includes `run`. The result in a running stack is not tested yet. `SaveToFile` may also fail with `EPERM` on `f.Chmod` when `server.conf` belongs to root.
3. Decide whether armv6 stays dropped. `arm32v6/golang:1.26.10-alpine` exists, but `build/Dockerfile-beego` needs Debian packages, so using it requires an Alpine path. `arm32v5/golang` has no current image.
4. Run a code review and commit `openvpn-server`.
