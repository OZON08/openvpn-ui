# Management interface over a Unix socket

Status: 2026-10-31. The change is committed on the branch `feat/management-unix-socket`. The complete stack was tested with both images built from `Dockerfile.ci` and the `openvpn-server` Dockerfile.

## Goal

The UI controls OpenVPN through `/etc/openvpn/run/mi.sock` instead of TCP port 2080. The interface has no password, so anyone who reached the port could read client data, disconnect clients or stop the server.

The matching server change is in the `openvpn-server` repo, version v0.6.4, in `docs/management-unix-socket.md`.

## Changes in this repo

| File | Change |
|---|---|
| `conf/app.conf`, `build/assets/app.conf` | Management address and network set to the socket |
| `conf/openvpn-server-config.tpl`, `controllers/ovconfig.go`, `models/settings.go` | The `management` line now comes from the settings. It was fixed to `0.0.0.0 2080` before. A socket path must be absolute and clean, and TCP needs an IP address and port |
| `controllers/base.go`, `settings.go`, `ovconfig.go`, `ovclientconfig.go`, `easyrsaconfig.go`, `dangerzone.go` | `RequireAdmin` limits these pages to administrators, as the menu already does |
| `.github/workflows/` | Actions are pinned to commit SHAs, because they receive the Docker Hub token |
| `models/models.go`, `models/management_test.go` | Stored TCP settings with port 2080 move to the socket at startup, once the `run` directory exists. Other ports are kept |
| `controllers/api-signal.go` | The signal API is limited to administrators and only accepts `SIGUSR1`, `SIGUSR2` and `SIGTERM` |
| `controllers/certificates.go` | Client profiles use `clientconfig.SaveToFile`, so they are validated as well |
| `build/assets/genclient.sh` | Removed a `chmod` on `pki/issued`, which stopped the script for a non-owner |
| `build/go.mod`, `build/go.sum` | Removed. They were outdated and only filled a cache layer in `build/Dockerfile-beego` |
| `views/settings.html` | Placeholders show the socket |
| `docs/docker-compose.yml`, `docs/docker-compose.influx.yml` | New `./run:/etc/openvpn/run` volume, healthcheck checks socket and process, port entry removed, `privileged` replaced by `/dev/net/tun` and the forwarding sysctl as in `openvpn-server`, `start_period` of 7200s |
| `go.mod`, `go.sum`, `vendor/` | `openvpn-server-config` v0.5.0, Go 1.26.10 and security updates for `x/net`, `x/text`, `grpc`, `go-ntlmssp` and `klauspost/compress` |
| `build/build.sh`, `build/Dockerfile-beego`, `Dockerfile.ci`, `build/standalone-install.sh` | Go 1.26.10. `build/build.sh` rejects armv5 and armv6 |
| `build/assets/start.sh` | Adds `openvpn-ui` to the group `openvpn-mi`, limits group access in `/etc/openvpn` to the paths the UI manages, hands the rendered files to `openvpn-ui`, and no longer reads its path from `conf/app.conf`, which the web user can change. `OPENVPN_DIR` overrides the default `/etc/openvpn` |
| `build/Dockerfile` | Installs `su-exec` and creates the user `openvpn-ui`, which `start.sh` needs |
| `CHANGELOG.md` | Entry for this change |
| `README.md`, `DOCKER_HUB.md` | `./run` volume in the examples |
| `README.md`, `docs/superpowers/plans/2026-06-20-statuslog-tests.md` | Real name removed |

These files are not part of this change and must stay out of its commit:

- `static/css/v097-custom.css`
- the untracked plans in `docs/superpowers/plans/`: `2026-06-14-grafana-dashboards.md`, `2026-06-20-auto-theme.md`, `2026-06-20-light-dark-toggle.md`

## How access is restricted

The socket has mode `0777`, so its own permissions protect nothing. Access is limited by the `run` directory, which the server container sets to owner root, GID `61194` and mode `0750` at every start.

`build/assets/start.sh` creates the group `openvpn-mi` with this GID and adds the user `openvpn-ui` to it, so the web process stays unprivileged and still reaches the socket. A separate GID is used because GID 1000 often belongs to a normal host user. A group that already has this GID is reused.

To use another GID, set `OPENVPN_MI_GID` to the same value on both services. Recreate the UI container after changing it. A host user who is a member of this GID can control OpenVPN, so it must not be assigned to anyone else.

## File ownership

`SaveToFile` sets a mode of `0600`, and only the owner of a file may do this. `start.sh` therefore gives `server.conf`, `config/client.conf`, `config/easy-rsa.vars`, `pki/vars` and `clients/` to UID 1000 at startup. Otherwise saving fails with `operation not permitted` when these files belong to root. On the host, these files then belong to the user with UID 1000, if there is one.

## Group access in `/etc/openvpn`

`start.sh` gives the group `openvpn-ui` (GID 1000) read and write access to `server.conf`, `config/`, `clients/`, `staticclients/` and `pki/`. It removes group write from everything else except `run`, also where earlier versions added it. The web process can therefore no longer change `fw-rules.sh`, `.env` or `/etc/openvpn` itself.

The certificate scripts run as the web user and need the CA key. **A host user with GID 1000 can therefore read `pki/private/ca.key` and the other private keys.** Before this change that user could overwrite these files but not read them, and creating certificates failed unless the README's `g+rX` step was applied.

## Rollout

1. Update the `openvpn` and `openvpn-ui` images together. The new UI only talks to the socket. An older UI opens `run` to GID 1000 when it starts, and so to a host user with this GID.
2. Add `./run:/etc/openvpn/run` to the `openvpn` service. Without this volume an existing config stays on TCP.
3. Use the new healthcheck from the compose files. The old one tests port 2080 and would fail.

The UI keeps its TCP settings until the `run` directory exists, so it stays on the same endpoint as the server. To go back to an older image after the migration, set the management address back to the old value in the settings.

## Behaviour changes from `openvpn-server-config` v0.5.0

- **EasyRSA values:** only letters, digits, spaces and `. , _ @ & + ( ) : -` are allowed. Saving fails for stored values with other characters, for example an apostrophe.
- **OpenVPN configs:** line breaks are rejected in single-value fields. Key fields must not contain `<`.
- **File modes:** `SaveToFile` sets existing files to `0600`. The file is updated in place, so the single-file mount of `server.conf` keeps working.
- **`SIGHUP`:** OpenVPN exits on this signal when `server.conf` has mode `0600`, because it runs as `nobody` and cannot read the file again. With `restart: always` the container restarts. The UI buttons use `SIGUSR1`, and the signal API now rejects `SIGHUP`.

## Tested

- Tests with `-race` and the build pass for `models`, `controllers` and `lib`.
- `govulncheck ./...` reports 0 vulnerabilities in called code. It still lists GO-2026-5932 in `x/crypto/openpgp`, which has no fix. That package is not in `vendor/`.
- The UI templates render with v0.5.0 using default values.
- The socket was tested with OpenVPN 2.6 in Docker, a client built from the library and the start block of `docker-entrypoint.sh`:
  - `SIGUSR1` and a container restart keep the socket working.
  - The healthcheck fails without the OpenVPN process.
- The complete stack was tested with both real images, using 2048-bit DH parameters and without `docker.sock`:
  - The UI runs as `openvpn-ui` and shows the OpenVPN version it reads over the socket. This works with either start order and after restarting either container.
  - `run` stays at `0:61194` with mode `0750`. UID 1000 without the group, UID 65534 and the host user have no access.
  - A database with `openvpn:2080` and `tcp` moves to the socket.
  - An old `server.conf` moves to the socket and keeps its inode. A line with port `20800` is left alone.
  - Saving `server.conf` through the UI works, also when the file belonged to root. It keeps its inode, gets mode `0600`, and OpenVPN starts with the rendered file.

One check with the UI started first failed once and returned no version. It passed in the other complete runs and in 10 further requests over 5 restarts of this start order, without socket errors in the log. The cause of the single failure is unknown.

A custom GID through `.env` was tested on both services.

The stack results above are from before the changes to group access, the `management` line and the signal API.

The new `start.sh` and `genclient.sh` were tested in the UI image, mounted over the older files, without the server container and with a test PKI that uses `EASYRSA_DN=cn_only`. As the web user:

- Creating and revoking a client certificate works. With the older files it fails, because the web user cannot read the PKI.
- `fw-rules.sh`, `.env` and `/etc/openvpn` are not writable. With the older files they are.
- `OPENVPN_MI_GID=1000` and a changed GID start correctly. With the older file, GID 1000 stops the container.

## Not tested

- An image built with `build/build.sh`, including the `build/Dockerfile` change.
- The restart buttons that use `docker.sock`.
- An actual VPN client connection with this version of the UI.
- Creating a certificate through the web interface against a PKI from the server container.
- Rootless Docker.

## Decisions

- armv6 stays dropped. CI builds only amd64, arm64 and armv7, and `build/Dockerfile-beego` needs Debian packages, so an Alpine Go image would require an additional build path.

## Open

1. The monitoring cannot read `log/openvpn-status.log` with an older server image: OpenVPN creates it with mode `0600` as root. `openvpn-server` now creates the logs for the group `openvpn-mi`. This is not tested with this version of the UI.
2. Decide which TLS mode client profiles use. `build/assets/genclient.sh`, `revoke.sh` and `rmcert.sh` write `<tls-auth>`, while `conf/openvpn-client-config.tpl` writes `<tls-crypt>`. A server config rendered by the UI reportedly rejects `tls-auth` clients. This does not come from this change.
3. A server config saved with TCP settings opens an unauthenticated port on the stored IP address. This only applies to settings an administrator kept on TCP. Host names are rejected, so saving fails until such settings use an IP address or the socket. This also applies when the `run` volume is still missing.
4. `conf/app.conf` and installs without containers point at the socket, but only the server image creates `run`. There, set the address and network in the settings.
5. Forms have no CSRF protection, because `EnableXSRF` is not set. A logged-in administrator who opens a malicious page could trigger changes. This does not come from this change.
6. The compose examples in `README.md` and `DOCKER_HUB.md` still use `privileged: true`.
