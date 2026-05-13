# OpenVPN UI

> **Security-hardened fork of [d3vilh/openvpn-ui](https://github.com/d3vilh/openvpn-ui), maintained by [OZON08](https://github.com/OZON08).**
> Critical/high vulnerabilities fixed, dependencies updated, build reliability improved.
> Full details in [CHANGELOG.md](https://github.com/OZON08/openvpn-ui/blob/main/CHANGELOG.md).

OpenVPN server web administration interface — quick to deploy, easy to use.

<img src="https://raw.githubusercontent.com/OZON08/openvpn-ui/main/docs/images/OpenVPN-UI-Home.png" alt="OpenVPN-UI home screen"/>

[![latest version](https://img.shields.io/github/v/release/OZON08/openvpn-ui?color=%2344cc11&label=LATEST%20RELEASE&style=flat-square&logo=Github)](https://github.com/OZON08/openvpn-ui/releases/latest)
[![Docker Image Version](https://img.shields.io/docker/v/ozon08/openvpn-ui/latest?logo=docker&label=DOCKER%20IMAGE&color=2344cc11&style=flat-square&logoColor=white)](https://hub.docker.com/r/ozon08/openvpn-ui)
![Docker Image Size](https://img.shields.io/docker/image-size/ozon08/openvpn-ui/latest?logo=Docker&color=2344cc11&label=IMAGE%20SIZE&style=flat-square&logoColor=white)

## Features

- Status page with server statistics and connected clients list
- Supports **tunnel** (`dev tun`) and **bridge** (`dev tap`) server configurations
- Generate, download, renew, revoke, delete and view client certificates
- Client **passphrase** and **static IP** assignment during certificate generation
- Two-factor authentication (**2FA/MFA**) support
- Change EasyRSA vars including certificate and CRL expiration time
- Maintain EasyRSA PKI infrastructure (init, build-ca, gen-dh, build-crl, gen-ta, revoke)
- Change OpenVPN Server configuration via web interface
- Preview OpenVPN Server logs
- Restart OpenVPN Server and OpenVPN UI from web interface
- User management — Admins have full access, regular users access certificates, logs and status only
- Admin credentials passed via environment variables
- Google OAuth 2.0 login support
- Alpine Linux base, Go 1.25.0, Beego 2.3.10, EasyRSA 3.x, OpenSSL 3.x
- **v0.9.7.2**: Security hardening — container now runs as a dedicated non-root user (`openvpn-ui`, UID/GID 1000) via `su-exec`. Mitigates privilege-escalation impact from host kernel vulnerabilities (e.g. CVE-2026-31431). Group write permissions ensure EasyRSA scripts continue to function without root.
- **v0.9.7.1**: Bugfix — certificate create/revoke/renew/remove actions failed inside the container because the `easyrsa` helper scripts read `EasyRsaPath`/`OpenVpnPath` via a broken relative path. Paths are now injected as env vars from Go. Dockerfile COPY order also corrected so the container-variant `app.conf` is not overwritten by the dev-time copy. Light/dark toggle hidden in the topbar (dark mode forced).
- **v0.9.7**: Long-term user monitoring — per-user transfer volume, real/virtual IPs and connect/disconnect sessions persisted in SQLite with retention policy, with optional InfluxDB v3 export for Grafana dashboards. New **Monitor** page (Sessions / Users / Retention / InfluxDB tabs, admin-editable InfluxDB settings with hot-reload) + `/api/v1/monitor/*` endpoints + optional OpenVPN `client-disconnect` webhook. UI refresh: AdminLTE sidebar layout with dark-mode default.
- **v0.9.6.1**: CRLF line ending fix — container now starts correctly on all Linux hosts
- AMD64 and ARM images available

## Security fixes in this fork

| Severity | Issue |
|----------|-------|
| Critical | Command injection in certificate/PKI management |
| Critical | Path traversal in certificate download endpoint |
| Critical | Path traversal in image display endpoint |
| Critical | CSRF via hardcoded OAuth state parameter |
| High | OAuth internal errors exposed to unauthenticated users |
| High | LDAP injection in authentication |
| High | SQL query logging of sensitive data in production |
| Medium | Race condition on global configuration state |
| Medium | World-readable client config files containing private keys |
| Medium | Weak password policy (minimum length raised 6 → 12) |
| Low | World-readable static IP config files |

## Quick Start

### docker-compose (recommended)

```yaml
---
version: "3.5"

services:
  openvpn:
    container_name: openvpn
    image: ozon08/openvpn-server:latest
    privileged: true
    ports:
      - "1194:1194/udp"
    environment:
      TRUST_SUB: 10.0.70.0/24
      GUEST_SUB: 10.0.71.0/24
      HOME_SUB: 192.168.88.0/24
    volumes:
      - ./pki:/etc/openvpn/pki
      - ./clients:/etc/openvpn/clients
      - ./config:/etc/openvpn/config
      - ./staticclients:/etc/openvpn/staticclients
      - ./log:/var/log/openvpn
      - ./fw-rules.sh:/opt/app/fw-rules.sh
      - ./server.conf:/etc/openvpn/server.conf
    cap_add:
      - NET_ADMIN
    restart: always

  openvpn-ui:
    container_name: openvpn-ui
    image: ozon08/openvpn-ui:latest
    environment:
      - OPENVPN_ADMIN_USERNAME=admin
      - OPENVPN_ADMIN_PASSWORD=changeme
    privileged: true
    ports:
      - "8080:8080/tcp"
    volumes:
      - ./:/etc/openvpn
      - ./db:/opt/openvpn-ui/db
      - ./pki:/usr/share/easy-rsa/pki
      - /var/run/docker.sock:/var/run/docker.sock:ro
    restart: always
```

```shell
git clone https://github.com/OZON08/openvpn-server ~/openvpn-server
cd ~/openvpn-server
docker compose up -d
```

### docker run

```shell
docker run \
  -v /etc/openvpn:/etc/openvpn \
  -v /etc/openvpn/db:/opt/openvpn-ui/db \
  -v /etc/openvpn/pki:/usr/share/easy-rsa/pki \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e OPENVPN_ADMIN_USERNAME='admin' \
  -e OPENVPN_ADMIN_PASSWORD='changeme' \
  -p 8080:8080/tcp \
  --privileged ozon08/openvpn-ui:latest
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `OPENVPN_ADMIN_USERNAME` | Admin username (set on first start) |
| `OPENVPN_ADMIN_PASSWORD` | Admin password — min 12 characters (set on first start) |
| `GOOGLE_CLIENT_ID` | Google OAuth 2.0 client ID (optional) |
| `GOOGLE_CLIENT_SECRET` | Google OAuth 2.0 client secret (optional) |
| `GOOGLE_REDIRECT_URL` | Google OAuth 2.0 redirect URL (optional) |
| `ALLOWED_DOMAINS` | Comma-separated list of allowed email domains for OAuth (optional) |
| `APP_ENV` | Set to `development` to enable ORM debug logging (default: production) |

## Configuration

OpenVPN UI is accessible at `http://localhost:8080` (default).

After the first start:
1. Log in with the credentials set via environment variables.
2. Go to **Configuration > OpenVPN Server** and set your server options.
3. Go to **Configuration > EasyRSA vars** and review PKI settings.
4. Unset `OPENVPN_ADMIN_USERNAME` / `OPENVPN_ADMIN_PASSWORD` env vars after the first successful login.

## Upgrade

```shell
docker pull ozon08/openvpn-ui:latest
docker rm openvpn-ui --force
docker compose up -d
```

The database schema is updated automatically on first start after upgrade.

## Links

- **GitHub:** [OZON08/openvpn-ui](https://github.com/OZON08/openvpn-ui)
- **OpenVPN Server image:** [OZON08/openvpn-server](https://hub.docker.com/r/ozon08/openvpn-server)
- **Changelog:** [CHANGELOG.md](https://github.com/OZON08/openvpn-ui/blob/main/CHANGELOG.md)
- **License:** [MIT](https://github.com/OZON08/openvpn-ui/blob/main/LICENSE)

[!["Buy Me A Coffee"](https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png)](https://www.buymeacoffee.com/ozon)
