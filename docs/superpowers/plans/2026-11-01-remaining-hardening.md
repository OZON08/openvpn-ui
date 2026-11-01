# Remaining hardening and UI languages Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the open items of `docs/management-unix-socket.md` and give the UI German and English.

**Architecture:** Forged requests are rejected centrally in `BaseController`. State changes use `POST`. Translations live in one file per language and go through one template function.

**Tech Stack:** Go, Beego v2, Bootstrap/AdminLTE, Docker Compose.

**Spec:** the Open list in `docs/management-unix-socket.md`.

## Global Constraints

- Plain HTTP on port 8080 must keep working. Beego's `EnableXSRF` sets a `Secure` cookie and cannot be used.
- No secrets in code. No real names. Source and comments in English.
- A failed token check answers 403 and changes nothing.

## Review Focus

- A form that lost its token: every `<form method="post">` must carry `xsrfdata`.
- A reload after an action must not repeat it (redirect after `POST`).
- A non-administrator calling user management or the container restart directly.
- The disconnect hook authenticates by token and must keep working without a session.
- A narrowed mount must keep the maintenance actions working, or stay as it is.

---

### Task 1: CSRF protection and POST actions

**Files:** `controllers/base.go`, `controllers/csrf.go`, `controllers/csrf_test.go`, `controllers/certificates.go`, `controllers/dangerzone.go`, `controllers/profile.go`, `controllers/api-base.go`, `main.go`, `routers/*`, `views/*`, `static/js/custom.js`

- [ ] Test `validCSRFToken(want, got string) bool` and `isSafeMethod(method string) bool`
- [ ] Token per session, checked in `BaseController.Prepare` when `EnableXSRF` is set; the hook and the Grafana proxy keep it off
- [ ] Session cookie `SameSite=Lax`
- [ ] Certificates, PKI, container, user delete and logout use `POST` and redirect afterwards
- [ ] Administrator checks on user management and the container restart
- [ ] Stack test: `GET` changes nothing, missing token gives 403, hook still works

### Task 2: Compose, examples and Dependabot

**Files:** `docs/docker-compose*.yml`, `README.md`, `DOCKER_HUB.md`, `.github/dependabot.yml`

- [ ] Examples without `privileged`
- [ ] UI mounts only the paths it manages, tested with the maintenance actions
- [ ] Dependabot covers `gomod`, `github-actions` and `docker`

### Task 3: Untested paths

- [ ] Image from `build/build.sh`, restart buttons with `docker.sock`, rootless Docker if available, install without containers
- [ ] TLS field in a browser, light and dark, keyboard

### Task 4: German and English

**Files:** `lib/i18n.go`, `lib/i18n_test.go`, `conf/locale/de.json`, `views/*`, `controllers/base.go`

- [ ] Test `lib.Translate(lang, text string) string` with fallback to the English text
- [ ] Language from cookie, then `Accept-Language`; selector in the top bar; `<html lang>` follows it
- [ ] All views use the template function, checked in a browser

### Task 5: Issue #1 in `openvpn-server`

- [ ] Find the cause of the shared IP in the graph with `duplicate-cn`, and assess `tls-crypt-v2`
