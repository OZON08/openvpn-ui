# Audit Log Design

## Goal

Persist a tamper-evident record of admin actions in SQLite, purge entries older
than a configurable retention window, and surface the log on a dedicated
admin-only page in the sidebar.

## Scope

### Events logged

| Action constant | Trigger |
|---|---|
| `login` | Successful password or Google OAuth login |
| `logout` | Explicit logout |
| `cert.create` | New certificate issued |
| `cert.revoke` | Certificate revoked |
| `cert.renew` | Certificate renewed |
| `cert.burn` | Certificate deleted (files removed) |
| `settings.save` | OpenVPN UI settings saved |
| `ovconfig.save` | OpenVPN server config saved |
| `influx.save` | InfluxDB configuration saved |
| `pki.delete` | Full PKI wiped via Danger Zone |
| `user.create` | New user account created |
| `user.update` | User account updated (name, password, admin flag) |
| `user.delete` | User account deleted |
| `cert.assign` | Certificate assigned to a user |
| `cert.unassign` | Certificate removed from a user |

Login failures are **not** logged (no authenticated actor, high noise from scanners).

### Out of scope

- Read-only actions (GET requests)
- Non-admin user actions (downloads, profile view)
- Log export or search/filter UI (v1 keeps it simple)
- Separate audit log file on disk

---

## Architecture

### Data model — `models/auditlog.go`

```go
type AuditLog struct {
    Id        int64     `orm:"auto"`
    CreatedAt time.Time `orm:"auto_now_add;type(datetime)"`
    UserID    int64     // 0 for system-level events
    UserLogin string    // snapshot of login name at time of action
    Action    string    // dot-namespaced constant, e.g. "cert.create"
    Target    string    // cert name, username, or "" when not applicable
    Detail    string    // optional free-text (e.g. cert CN on create)
    IPAddr    string    // client IP from c.Ctx.Input.IP()
}
```

Registered in `models.InitDB()` alongside the other models; `orm.RunSyncdb`
auto-creates the `audit_log` table on first run.

### Helper function

```go
// WriteAuditLog persists one audit entry. Safe to call with a nil/zero actor
// (login events where Userinfo is not yet available use userID=0).
func WriteAuditLog(userID int64, userLogin, action, target, detail, ip string) {
    entry := &AuditLog{
        UserID:    userID,
        UserLogin: userLogin,
        Action:    action,
        Target:    target,
        Detail:    detail,
        IPAddr:    ip,
    }
    o := orm.NewOrm()
    if _, err := o.Insert(entry); err != nil {
        logs.Error("audit log write failed:", err)
    }
}
```

### Retention cleanup

A goroutine started in `main.go` (alongside the monitor scraper) runs a daily
ticker. On each tick it deletes rows where
`created_at < now - retention_days`:

```go
func RunAuditLogRetention(retentionDays int) {
    ticker := time.NewTicker(24 * time.Hour)
    for range ticker.C {
        cutoff := time.Now().AddDate(0, 0, -retentionDays)
        o := orm.NewOrm()
        _, err := o.Raw("DELETE FROM audit_log WHERE created_at < ?", cutoff).Exec()
        if err != nil {
            logs.Error("audit log retention cleanup failed:", err)
        }
    }
}
```

Configuration: `AuditLogRetentionDays` in `conf/app.conf` and
`build/assets/app.conf`, overridable via
`OPENVPN_UI_AUDITLOG_RETENTION_DAYS` environment variable (default: 365).

---

## Controller integration

`WriteAuditLog` is called **after** a successful action, never on error paths.
Actor info comes from `c.Userinfo` (available on all controllers via
`BaseController`). For login events, `c.Userinfo` is not yet set — use the
submitted username and `userID = 0`.

### Callsites

| File | Method | action | target |
|---|---|---|---|
| `controllers/login.go` | `Login()` success branch | `login` | `""` |
| `controllers/login.go` | `Logout()` | `logout` | `""` |
| `controllers/login.go` | `GoogleCallback()` success branch | `login` | `""` |
| `controllers/certificates.go` | `Post()` after cert written | `cert.create` | cert name |
| `controllers/certificates.go` | `Revoke()` after revoke | `cert.revoke` | cert name |
| `controllers/certificates.go` | `Renew()` after renew | `cert.renew` | cert name |
| `controllers/certificates.go` | `Burn()` after delete | `cert.burn` | cert name |
| `controllers/settings.go` | `Post()` after save | `settings.save` | `""` |
| `controllers/ovconfig.go` | `Post()` after save | `ovconfig.save` | `""` |
| `controllers/monitor.go` | `SaveInflux()` after save | `influx.save` | `""` |
| `controllers/dangerzone.go` | `DeletePKI()` after delete | `pki.delete` | `""` |
| `controllers/profile.go` | `Post()` — create branch | `user.create` | username |
| `controllers/profile.go` | `Post()` — update branch | `user.update` | username |
| `controllers/profile.go` | `DeleteUser()` after delete | `user.delete` | username |
| `controllers/profile.go` | `AssignCert()` after assign | `cert.assign` | cert name |
| `controllers/profile.go` | `RemoveCert()` after remove | `cert.unassign` | cert name |

IP address: `c.Ctx.Input.IP()` on all controller callsites; for login events
`c.Ctx.Input.IP()` is available before the session is established.

---

## UI

### Controller — `controllers/auditlog.go`

`AuditLogController` embeds `BaseController`. `NestPrepare` checks login and
admin status, calls `c.StopRun()` on failure (same pattern as
`APIMonitorRetentionController`). `Get()` reads a `?page=N` query param
(default 1), queries the DB for 50 entries ordered `created_at DESC` with
`LIMIT 50 OFFSET (page-1)*50`, passes total count and entries to the template.

### View — `views/auditlog.html`

Standard AdminLTE card layout (same structure as `monitor.html`):
- Table columns: **Time** (local, formatted `2006-01-02 15:04`), **User**,
  **Action**, **Target**, **Detail**, **IP**
- Pagination links at the bottom (Previous / Next, page N of M)
- No search or filter controls in v1

### Route

```go
web.Router("/auditlog", &controllers.AuditLogController{})
```

### Sidebar entry

In `views/common/sidebar.html`, inside the `{{if .Userinfo.IsAdmin}}` block,
between Monitor and the Configuration nav-header:

```html
<li class="nav-item">
  <a href="{{urlfor "AuditLogController.Get"}}"
     class="nav-link {{if compare .RouterPattern "/auditlog"}}active{{end}}">
    <i class="nav-icon fa fa-list-alt"></i>
    <p>Audit Log</p>
  </a>
</li>
```

---

## Files changed

| File | Change |
|---|---|
| `models/auditlog.go` | New — `AuditLog` model + `WriteAuditLog()` + `RunAuditLogRetention()` |
| `models/models.go` | Register `new(AuditLog)` in `orm.RegisterModel` |
| `main.go` | Read retention config, start `models.RunAuditLogRetention()` goroutine |
| `conf/app.conf` | Add `AuditLogRetentionDays = 365` |
| `build/assets/app.conf` | Add `AuditLogRetentionDays = ${OPENVPN_UI_AUDITLOG_RETENTION_DAYS||365}` |
| `controllers/auditlog.go` | New — `AuditLogController` |
| `controllers/login.go` | Add `WriteAuditLog` calls in `Login`, `Logout`, `GoogleCallback` |
| `controllers/certificates.go` | Add `WriteAuditLog` calls in `Post`, `Revoke`, `Renew`, `Burn` |
| `controllers/settings.go` | Add `WriteAuditLog` call in `Post` |
| `controllers/ovconfig.go` | Add `WriteAuditLog` call in `Post` |
| `controllers/monitor.go` | Add `WriteAuditLog` call in `SaveInflux` |
| `controllers/dangerzone.go` | Add `WriteAuditLog` call in `DeletePKI` |
| `controllers/profile.go` | Add `WriteAuditLog` calls in `Post`, `DeleteUser`, `AssignCert`, `RemoveCert` |
| `views/auditlog.html` | New — paginated table view |
| `views/common/sidebar.html` | Add Audit Log entry in admin block |
| `routers/router.go` | Register `/auditlog` route |
