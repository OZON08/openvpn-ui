# Per-User Certificate and Log Scoping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restrict non-admin users to seeing only their explicitly assigned certificates and matching log lines, using a `user_certificates` join table in SQLite. Admin can assign, remove, and transfer certs between users; a seed action seeds assignments from existing login names for smooth upgrades.

**Architecture:** A new `UserCertificate` ORM model (user_id + cert_name) is auto-migrated at startup. Admin manages assignments via a new Profile tab (assign, remove, transfer, seed). `CertificatesController` filters the cert list and guards all cert actions against non-owners; `LogsController` filters log lines by matching CN patterns against the user's assigned cert names.

**Tech Stack:** Beego v2 ORM (SQLite, auto-migrated via `RunSyncdb`), Go `text/template` (Beego's template engine), Bootstrap 4 tab + badge UI.

---

### Task 1: Add `UserCertificate` model

**Files:**
- Create: `models/user_certificate.go`
- Modify: `models/models.go` lines 35–46

- [ ] **Step 1: Create `models/user_certificate.go`**

```go
package models

import "github.com/beego/beego/v2/client/orm"

type UserCertificate struct {
	Id       int64  `orm:"auto"`
	UserId   int64  `orm:"index"`
	CertName string `orm:"size(128)"`
}

func CertsForUser(userID int64) ([]string, error) {
	var rows []UserCertificate
	_, err := orm.NewOrm().QueryTable(new(UserCertificate)).
		Filter("UserId", userID).All(&rows)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, r.CertName)
	}
	return names, nil
}

func AssignCert(userID int64, certName string) error {
	exists := orm.NewOrm().QueryTable(new(UserCertificate)).
		Filter("UserId", userID).Filter("CertName", certName).Exist()
	if exists {
		return nil
	}
	_, err := orm.NewOrm().Insert(&UserCertificate{UserId: userID, CertName: certName})
	return err
}

func RemoveCert(userID int64, certName string) error {
	_, err := orm.NewOrm().QueryTable(new(UserCertificate)).
		Filter("UserId", userID).Filter("CertName", certName).Delete()
	return err
}

// TransferCert atomically removes a cert assignment from one user and assigns it
// to another. If toUserID already has the assignment, only the source is removed.
func TransferCert(fromUserID, toUserID int64, certName string) error {
	o := orm.NewOrm()
	if err := o.Begin(); err != nil {
		return err
	}
	if _, err := o.QueryTable(new(UserCertificate)).
		Filter("UserId", fromUserID).Filter("CertName", certName).Delete(); err != nil {
		_ = o.Rollback()
		return err
	}
	if !o.QueryTable(new(UserCertificate)).
		Filter("UserId", toUserID).Filter("CertName", certName).Exist() {
		if _, err := o.Insert(&UserCertificate{UserId: toUserID, CertName: certName}); err != nil {
			_ = o.Rollback()
			return err
		}
	}
	return o.Commit()
}

// SeedAssignmentsFromLogin creates cert assignments for non-admin users where
// user.Login matches a cert CN. Skips existing assignments. Returns count created.
// Used as a one-time migration aid when upgrading existing systems.
func SeedAssignmentsFromLogin(certNames []string) (int, error) {
	o := orm.NewOrm()
	var users []User
	if _, err := o.QueryTable(new(User)).Filter("IsAdmin", false).All(&users); err != nil {
		return 0, err
	}
	certSet := make(map[string]bool, len(certNames))
	for _, n := range certNames {
		certSet[n] = true
	}
	created := 0
	for _, u := range users {
		if !certSet[u.Login] {
			continue
		}
		if err := AssignCert(u.Id, u.Login); err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}
```

- [ ] **Step 2: Register model in `models/models.go`**

Find the `orm.RegisterModel(...)` block (lines 35–46). Add `new(UserCertificate)` as the last entry:

```go
	orm.RegisterModel(
		new(User),
		new(Settings),
		new(OVConfig),
		new(OVClientConfig),
		new(EasyRSAConfig),
		new(VpnSession),
		new(TrafficSample),
		new(TrafficHourly),
		new(TrafficDaily),
		new(InfluxSettings),
		new(UserCertificate),
	)
```

`RunSyncdb` already runs at startup (`models/models.go` line 48) — it will auto-create the `user_certificate` table on next launch.

- [ ] **Step 3: Verify build**

```
go build ./...
```

Expected: no output (clean build).

- [ ] **Step 4: Commit**

```bash
git add models/user_certificate.go models/models.go
git commit -m "feat: add UserCertificate model with assign/remove/transfer/seed"
git push
```

---

### Task 2: Admin UI — assign, remove, transfer, and seed certs in Profile page

**Files:**
- Modify: `controllers/profile.go` — extend `Get()`, add `AssignCert()`, `RemoveCert()`, `TransferCert()`, `SeedCerts()` methods
- Modify: `routers/router.go` — add 4 new routes
- Modify: `views/profile.html` — add 4th admin-only tab

- [ ] **Step 1: Add `UserCertRow` type and extend imports in `controllers/profile.go`**

Add a package-level type before the `ProfileController` struct (after the `NewUser` struct, around line 23):

```go
// UserCertRow is the template payload for one row of the cert assignment tab.
type UserCertRow struct {
	User  *models.User
	Certs []string
}
```

Add the two missing imports to the import block (top of file):

```go
	"path/filepath"
	"github.com/OZON08/openvpn-ui/state"
```

(`lib` is already imported.)

- [ ] **Step 2: Extend `ProfileController.Get()` to load cert assignment data**

Replace the current `Get()` method (lines 39–55) with:

```go
func (c *ProfileController) Get() {
	c.Data["xsrfdata"] = template.HTML(c.XSRFFormHTML())
	c.Data["profile"] = c.Userinfo
	c.TplName = "profile.html"

	if c.Userinfo.IsAdmin {
		o := orm.NewOrm()
		var users []*models.User
		if _, err := o.QueryTable("user").All(&users); err != nil {
			logs.Error("Failed to retrieve user profiles:", err)
			return
		}
		c.Data["users"] = users

		var assignments []UserCertRow
		for _, u := range users {
			certs, _ := models.CertsForUser(u.Id)
			assignments = append(assignments, UserCertRow{User: u, Certs: certs})
		}
		c.Data["userCertAssignments"] = assignments

		pkiIndex := filepath.Join(state.GlobalCfg.OVConfigPath, "pki/index.txt")
		allCerts, err := lib.ReadCerts(pkiIndex)
		if err == nil {
			var certNames []string
			for _, cert := range allCerts {
				if cert.Details != nil && cert.Details.Name != "server" && cert.EntryType == "V" {
					certNames = append(certNames, cert.Details.Name)
				}
			}
			c.Data["allCertNames"] = certNames
		}
	}
}
```

- [ ] **Step 3: Add `AssignCert()` to `ProfileController`**

Append to `controllers/profile.go`:

```go
func (c *ProfileController) AssignCert() {
	if !c.Userinfo.IsAdmin {
		c.Ctx.Redirect(302, c.URLFor("MainController.Get"))
		return
	}
	flash := web.NewFlash()
	userID, err := c.GetInt64("userID")
	if err != nil {
		flash.Error("Invalid user ID")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	certName := c.GetString("certName")
	if !lib.SafeNameRegex.MatchString(certName) {
		flash.Error("Invalid certificate name")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	if err := models.AssignCert(userID, certName); err != nil {
		logs.Error("AssignCert failed:", err)
		flash.Error("Failed to assign certificate: %s", err.Error())
	} else {
		flash.Success("Certificate \"%s\" assigned successfully", certName)
	}
	flash.Store(&c.Controller)
	c.Redirect(c.URLFor("ProfileController.Get"), 302)
}
```

- [ ] **Step 4: Add `RemoveCert()` to `ProfileController`**

Append to `controllers/profile.go`:

```go
func (c *ProfileController) RemoveCert() {
	if !c.Userinfo.IsAdmin {
		c.Ctx.Redirect(302, c.URLFor("MainController.Get"))
		return
	}
	flash := web.NewFlash()
	userID, err := c.GetInt64(":userID")
	if err != nil {
		flash.Error("Invalid user ID")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	certName := c.GetString(":certName")
	if !lib.SafeNameRegex.MatchString(certName) {
		flash.Error("Invalid certificate name")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	if err := models.RemoveCert(userID, certName); err != nil {
		logs.Error("RemoveCert failed:", err)
		flash.Error("Failed to remove certificate: %s", err.Error())
	} else {
		flash.Success("Certificate \"%s\" removed", certName)
	}
	flash.Store(&c.Controller)
	c.Redirect(c.URLFor("ProfileController.Get"), 302)
}
```

- [ ] **Step 5: Add `TransferCert()` to `ProfileController`**

Append to `controllers/profile.go`:

```go
func (c *ProfileController) TransferCert() {
	if !c.Userinfo.IsAdmin {
		c.Ctx.Redirect(302, c.URLFor("MainController.Get"))
		return
	}
	flash := web.NewFlash()
	fromUserID, err := c.GetInt64("fromUserID")
	if err != nil {
		flash.Error("Invalid source user ID")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	toUserID, err := c.GetInt64("toUserID")
	if err != nil {
		flash.Error("Invalid target user ID")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	certName := c.GetString("certName")
	if !lib.SafeNameRegex.MatchString(certName) {
		flash.Error("Invalid certificate name")
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	if err := models.TransferCert(fromUserID, toUserID, certName); err != nil {
		logs.Error("TransferCert failed:", err)
		flash.Error("Transfer failed: %s", err.Error())
	} else {
		flash.Success("Certificate \"%s\" transferred successfully", certName)
	}
	flash.Store(&c.Controller)
	c.Redirect(c.URLFor("ProfileController.Get"), 302)
}
```

- [ ] **Step 6: Add `SeedCerts()` to `ProfileController`**

Append to `controllers/profile.go`:

```go
func (c *ProfileController) SeedCerts() {
	if !c.Userinfo.IsAdmin {
		c.Ctx.Redirect(302, c.URLFor("MainController.Get"))
		return
	}
	flash := web.NewFlash()
	pkiIndex := filepath.Join(state.GlobalCfg.OVConfigPath, "pki/index.txt")
	allCerts, err := lib.ReadCerts(pkiIndex)
	if err != nil {
		flash.Error("Could not read PKI: %s", err.Error())
		flash.Store(&c.Controller)
		c.Redirect(c.URLFor("ProfileController.Get"), 302)
		return
	}
	var certNames []string
	for _, cert := range allCerts {
		if cert.Details != nil && cert.EntryType == "V" {
			certNames = append(certNames, cert.Details.Name)
		}
	}
	n, err := models.SeedAssignmentsFromLogin(certNames)
	if err != nil {
		logs.Error("SeedCerts failed:", err)
		flash.Error("Seed failed: %s", err.Error())
	} else {
		flash.Success("Seeded %d assignment(s) from login names", n)
	}
	flash.Store(&c.Controller)
	c.Redirect(c.URLFor("ProfileController.Get"), 302)
}
```

- [ ] **Step 7: Add routes in `routers/router.go`**

After the existing `/profile` route (line 22), add:

```go
	web.Router("/profile/cert/assign",   &controllers.ProfileController{}, "post:AssignCert")
	web.Router("/profile/cert/remove/:userID/:certName", &controllers.ProfileController{}, "get:RemoveCert")
	web.Router("/profile/cert/transfer", &controllers.ProfileController{}, "post:TransferCert")
	web.Router("/profile/cert/seed",     &controllers.ProfileController{}, "post:SeedCerts")
```

- [ ] **Step 8: Add cert assignments tab to `views/profile.html`**

**8a.** In the tab nav (around line 15–16), after the "Managing Profiles" `<li>`, add:

```html
      {{ if .Userinfo.IsAdmin }} <li class="nav-item"><a class="nav-link" href="#tab_4" data-toggle="tab">Certificate Assignments</a></li> {{ end }}
```

**8b.** Find the closing `</div>` of the `tab-content` div (the one that wraps all `tab-pane` divs) and insert this new pane before it:

```html
      {{ if .Userinfo.IsAdmin }}
      <!--TAB4-->
      <div class="tab-pane" id="tab_4">
        {{template "common/alert.html" .}}

        <div class="d-flex align-items-center mb-4">
          <h3 class="mr-4 mb-0">Certificate Assignments</h3>
          <form method="post" action="/profile/cert/seed">
            {{.xsrfdata}}
            <button type="submit" class="btn btn-secondary btn-sm"
                    onclick="return confirm('Auto-assign certs to users whose login name matches a cert CN?\nExisting assignments are kept.')">
              <i class="fa fa-magic mr-1"></i>Seed from Login Names
            </button>
          </form>
        </div>
        <p class="text-muted">
          "Seed from Login Names" auto-assigns each cert to the user whose login matches the cert CN.
          Use this once after upgrading from a version without cert scoping.
        </p>

        <h4>Assign Certificate</h4>
        <form method="post" action="/profile/cert/assign">
          {{.xsrfdata}}
          <div class="form-row align-items-end mb-3">
            <div class="form-group col-md-4">
              <label>User</label>
              <select name="userID" class="form-control">
                {{range .users}}
                <option value="{{.Id}}">{{.Name}} ({{.Login}})</option>
                {{end}}
              </select>
            </div>
            <div class="form-group col-md-4">
              <label>Certificate</label>
              <select name="certName" class="form-control">
                {{range .allCertNames}}
                <option value="{{.}}">{{.}}</option>
                {{end}}
              </select>
            </div>
            <div class="form-group col-md-2">
              <button type="submit" class="btn btn-primary">Assign</button>
            </div>
          </div>
        </form>

        <h4>Current Assignments</h4>
        <table class="table table-bordered">
          <thead>
            <tr><th>User</th><th>Login</th><th>Assigned Certificates</th></tr>
          </thead>
          <tbody>
            {{range .userCertAssignments}}
            {{$row := .}}
            <tr>
              <td>{{$row.User.Name}}</td>
              <td>{{$row.User.Login}}</td>
              <td>
                {{range $row.Certs}}
                <span class="badge badge-secondary mr-1">{{.}}</span>
                <a href="/profile/cert/remove/{{$row.User.Id}}/{{.}}"
                   class="text-danger mr-2"
                   title="Remove"
                   onclick="return confirm('Remove {{.}} from {{$row.User.Name}}?')">
                  <i class="fa fa-times"></i>
                </a>
                {{else}}
                <span class="text-muted">none</span>
                {{end}}
              </td>
            </tr>
            {{end}}
          </tbody>
        </table>

        <h4 class="mt-4">Transfer Certificate</h4>
        <p class="text-muted">Moves a cert from one user to another in one atomic step.</p>
        <form method="post" action="/profile/cert/transfer">
          {{.xsrfdata}}
          <div class="form-row align-items-end mb-3">
            <div class="form-group col-md-3">
              <label>From User</label>
              <select name="fromUserID" class="form-control">
                {{range .users}}
                <option value="{{.Id}}">{{.Name}} ({{.Login}})</option>
                {{end}}
              </select>
            </div>
            <div class="form-group col-md-3">
              <label>Certificate</label>
              <select name="certName" class="form-control">
                {{range .allCertNames}}
                <option value="{{.}}">{{.}}</option>
                {{end}}
              </select>
            </div>
            <div class="form-group col-md-3">
              <label>To User</label>
              <select name="toUserID" class="form-control">
                {{range .users}}
                <option value="{{.Id}}">{{.Name}} ({{.Login}})</option>
                {{end}}
              </select>
            </div>
            <div class="form-group col-md-2">
              <button type="submit" class="btn btn-warning">Transfer</button>
            </div>
          </div>
        </form>

      </div>
      {{ end }}
```

- [ ] **Step 9: Verify build**

```
go build ./...
```

Expected: clean.

- [ ] **Step 10: Commit**

```bash
git add controllers/profile.go routers/router.go views/profile.html
git commit -m "feat: add cert assignment, transfer and seed UI to admin profile page"
git push
```

---

### Task 3: Filter certificates for non-admins

**Files:**
- Modify: `controllers/certificates.go` — `showCerts()`, `Download()`, `Revoke()`, `Burn()`, `Renew()`; add `canAccessCert()`
- Modify: `views/certificates.html` — hide create form, admin actions, and server control for non-admins

- [ ] **Step 1: Filter cert list and expose `IsAdmin` flag in `showCerts()`**

Replace `showCerts()` (lines 127–141) with:

```go
func (c *CertificatesController) showCerts() {
	path := filepath.Join(state.GlobalCfg.OVConfigPath, "pki/index.txt")
	certs, err := lib.ReadCerts(path)
	if err != nil {
		logs.Error(err)
	}
	lib.Dump(certs)

	if !c.Userinfo.IsAdmin {
		allowed, _ := models.CertsForUser(c.Userinfo.Id)
		allowSet := make(map[string]bool, len(allowed))
		for _, n := range allowed {
			allowSet[n] = true
		}
		filtered := certs[:0]
		for _, cert := range certs {
			if cert.Details != nil && allowSet[cert.Details.Name] {
				filtered = append(filtered, cert)
			}
		}
		certs = filtered
	}

	c.Data["certificates"] = &certs
	c.Data["IsAdmin"] = c.Userinfo.IsAdmin
	cfg := models.EasyRSAConfig{Profile: "default"}
	_ = cfg.Read("Profile")
	c.Data["EasyRSA"] = &cfg
	cfg1 := models.OVClientConfig{Profile: "default"}
	_ = cfg1.Read("Profile")
	c.Data["SettingsC"] = &cfg1
}
```

- [ ] **Step 2: Add `canAccessCert()` helper**

Append to `controllers/certificates.go`:

```go
func (c *CertificatesController) canAccessCert(name string) bool {
	if c.Userinfo.IsAdmin {
		return true
	}
	allowed, err := models.CertsForUser(c.Userinfo.Id)
	if err != nil {
		return false
	}
	for _, n := range allowed {
		if n == name {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Guard `Download()` with ownership check**

In `Download()` (starting at line 51), add after the `SafeNameRegex` check:

```go
	if !c.canAccessCert(name) {
		c.Ctx.Output.SetStatus(403)
		return
	}
```

The top of `Download()` becomes:

```go
func (c *CertificatesController) Download() {
	name := c.GetString(":key")
	if !lib.SafeNameRegex.MatchString(name) {
		c.Ctx.Output.SetStatus(400)
		return
	}
	if !c.canAccessCert(name) {
		c.Ctx.Output.SetStatus(403)
		return
	}
	// ... rest unchanged
```

- [ ] **Step 4: Restrict `Revoke()`, `Burn()`, and `Renew()` to admins**

At the top of each method's body, after `flash := web.NewFlash()`, add:

```go
	if !c.Userinfo.IsAdmin {
		c.Redirect(c.URLFor("CertificatesController.Get"), 302)
		return
	}
```

Apply this to `Revoke()` (line 176), `Burn()` (line 201), and `Renew()` (line 220).

- [ ] **Step 5: Hide admin-only sections in `views/certificates.html`**

**5a.** Wrap the Actions column cell (lines 105–122, the `<td class="align-middle">` block) with `{{if $.IsAdmin}}`. Replace those lines with:

```html
                  {{if $.IsAdmin}}
                    <td class="align-middle">
                  {{ if and (eq .Revocation "") (ne .Details.Name "") }}
                      <a href="{{urlfor "CertificatesController.Renew" ":key" .Details.Name ":localip" .Details.LocalIP ":serial" .Serial ":tfaname" .Details.TFAName}}" class="btn btn-primary btn-xs" title="Renew" style="font-size: 80%;">Renew</a>
                  {{else}}
                      <a class="btn btn-default btn-xs" disabled>Renew</a>
                  {{end}}
                  {{ if and (eq .Revocation "") (ne .Details.Name "") }}
                      <a href="{{urlfor "CertificatesController.Revoke" ":key" .Details.Name ":serial" .Serial ":tfaname" .Details.TFAName}}" class="btn btn-warning btn-xs" title="Revoke" style="font-size: 80%;">Revoke</a>
                  {{else}}
                      <a class="btn btn-default btn-xs" disabled>Revoke</a>
                  {{end}}
                  {{ if eq .Revocation ""}}
                      <a class="btn btn-default btn-xs" disabled>Delete</a>
                  {{else}}
                      <a href="{{urlfor "CertificatesController.Burn" ":key" .Details.CN ":serial" .Serial ":tfaname" .Details.TFAName}}" class="btn btn-danger btn-xs" title="Burn" style="font-size: 80%;">Delete</a>
                  {{end}}
                    </td>
                  {{else}}
                    <td></td>
                  {{end}}
```

**5b.** Wrap the "Create New Client Certificate" card (lines 208–228) with `{{if .IsAdmin}}..{{end}}`.

**5c.** Wrap the create certificate modal (lines 230–333, `<div class="modal fade" id="modal-default">`) with `{{if .IsAdmin}}..{{end}}`.

**5d.** Wrap the "OpenVPN Server Control" card (lines 335–352) with `{{if .IsAdmin}}..{{end}}`.

- [ ] **Step 6: Verify build**

```
go build ./...
```

Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add controllers/certificates.go views/certificates.html
git commit -m "feat: scope certificates page and actions to assigned users"
git push
```

---

### Task 4: Filter log lines for non-admins

**Files:**
- Modify: `controllers/logs.go`

- [ ] **Step 1: Add `lineMatchesCert()` helper**

Append after the closing brace of `Get()`:

```go
// lineMatchesCert reports whether a log line concerns one of the given cert names.
// Covers the main OpenVPN log patterns: VERIFY lines (CN=name,), peer connection
// lines ([name]), and routed-packet lines (name/ip:port).
func lineMatchesCert(line string, names []string) bool {
	for _, n := range names {
		if strings.Contains(line, "CN="+n+",") ||
			strings.Contains(line, "CN="+n+" ") ||
			strings.HasSuffix(line, "CN="+n) ||
			strings.Contains(line, "["+n+"]") ||
			strings.HasPrefix(line, n+"/") ||
			strings.Contains(line, " "+n+"/") {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Apply filtering in `LogsController.Get()`**

Replace `Get()` (lines 23–58) with:

```go
func (c *LogsController) Get() {
	c.TplName = "logs.html"
	c.Data["breadcrumbs"] = &BreadCrumbs{
		Title: "Logs",
	}

	settings := models.Settings{Profile: "default"}
	settings.Read("Profile")

	if err := settings.Read("OVConfigPath"); err != nil {
		logs.Error(err)
		return
	}

	fName := settings.OVConfigPath + "/log/openvpn.log"
	file, err := os.Open(fName)
	if err != nil {
		logs.Error(err)
		return
	}
	defer file.Close()

	var allowedCerts []string
	if !c.Userinfo.IsAdmin {
		allowedCerts, _ = models.CertsForUser(c.Userinfo.Id)
	}

	scanner := bufio.NewScanner(file)
	var logLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, " MANAGEMENT: ") {
			continue
		}
		if !c.Userinfo.IsAdmin && !lineMatchesCert(line, allowedCerts) {
			continue
		}
		logLines = append(logLines, strings.Trim(line, "\t"))
	}
	start := len(logLines) - 300
	if start < 0 {
		start = 0
	}
	c.Data["logs"] = logLines[start:]
}
```

- [ ] **Step 3: Verify build**

```
go build ./...
```

Expected: clean.

- [ ] **Step 4: Commit**

```bash
git add controllers/logs.go
git commit -m "feat: filter log lines to assigned cert names for non-admins"
git push
```

---

### Task 5: Migration, ROADMAP update, and manual test

**Files:**
- Modify: `ROADMAP.md`

- [ ] **Step 1 (pre-deploy migration): Seed assignments on existing systems**

**Before exposing the update to non-admin users**, do this as admin:

1. Deploy the updated container.
2. Log in as admin → Profile → **Certificate Assignments** tab.
3. Click **"Seed from Login Names"** — this auto-creates an assignment for every non-admin user whose `login` matches a valid cert CN. Flash message shows how many were created.
4. Review the "Current Assignments" table. For any user whose login does **not** match their cert CN (e.g., login `bob`, cert CN `bobby`), use the **Assign** form to manually link them.
5. Users with no assignments will see an empty cert list and filtered logs until an assignment exists. They can still connect to the VPN — only the UI is restricted.

- [ ] **Step 2: Mark item done in `ROADMAP.md`**

Find the per-user cert scoping entry and change `- [ ]` to `- [x]` and update the description:

```markdown
- [x] **Per-user certificate and log scoping** — non-admin users see only
      their assigned certificates and matching log lines. Cert names are
      mapped explicitly via the admin Profile → Certificate Assignments tab
      (`user_certificates` SQLite table, auto-migrated at startup). Admins
      retain full access. Migration: use the "Seed from Login Names" action
      to pre-fill assignments from existing `user.Login == cert.CN` pairs.
```

- [ ] **Step 3: Commit**

```bash
git add ROADMAP.md
git commit -m "docs: mark per-user cert and log scoping as done"
git push
```

- [ ] **Step 4: Manual test checklist**

Deploy and verify:

1. **Migration seed**: log in as admin → Profile → Certificate Assignments → click Seed → verify assignments appear in the table.

2. **Admin — full access**: Certificates shows all certs, Create form visible, Renew/Revoke/Delete buttons visible → Logs shows all lines.

3. **Non-admin — cert page**: log in as non-admin with one cert assigned → only that cert visible; no Create form, no action buttons, no Server Control section.

4. **Non-admin — download**: click the cert name → `.ovpn` file downloads correctly.

5. **Unauthorized download**: as non-admin, request `/certificates/<other-cert>` directly → 403 response.

6. **Non-admin — no assignments**: log in as non-admin with no assignments → empty cert table.

7. **Non-admin — logs**: only lines matching assigned cert CN patterns are visible.

8. **Remove assignment**: admin removes a cert badge → user can no longer download it (403).

9. **Transfer**: admin uses Transfer form (from user A, cert X, to user B) → cert X disappears from A's row and appears in B's row; user A gets 403 on download; user B can download.

10. **Transfer idempotent**: transfer a cert to a user who already has it → no error, cert stays assigned to target, removed from source.

---

## Self-Review

**Spec coverage:**
- ✅ `user_certificates` join table → Task 1
- ✅ Migration seed for existing systems → Task 1 (`SeedAssignmentsFromLogin`), Task 2 (UI button), Task 5 (pre-deploy steps)
- ✅ Admin assign cert to user → Task 2
- ✅ Admin remove cert from user → Task 2
- ✅ Admin transfer cert between users (atomic) → Task 1 (`TransferCert`), Task 2 (UI form)
- ✅ Non-admin cert list filtered → Task 3, Step 1
- ✅ `Download()` ownership-guarded → Task 3, Step 3
- ✅ `Revoke()` / `Burn()` / `Renew()` admin-only → Task 3, Step 4
- ✅ Create form + modal hidden for non-admins → Task 3, Step 5b/5c
- ✅ Server Control section hidden for non-admins → Task 3, Step 5d
- ✅ Non-admin log lines filtered by cert CN → Task 4

**Type consistency:**
- `UserCertificate` defined Task 1; `models.CertsForUser(int64)` called with `c.Userinfo.Id` (int64) in Tasks 3 & 4 — consistent.
- `models.AssignCert(int64, string)` / `models.RemoveCert(int64, string)` defined Task 1, called in Task 2 — consistent.
- `models.TransferCert(int64, int64, string)` defined Task 1, called in Task 2 — consistent.
- `models.SeedAssignmentsFromLogin([]string)` defined Task 1, called in Task 2 — consistent.
- `canAccessCert(name string) bool` defined and used in Task 3 — consistent.
- `lineMatchesCert(line string, names []string) bool` defined and used in Task 4 — consistent.
- `UserCertRow{User *models.User, Certs []string}` defined Task 2 Step 1; template accesses `.User.Name`, `.User.Login`, `.User.Id`, `.Certs` — all valid.
- `c.Data["IsAdmin"]` set in `showCerts()` (Task 3 Step 1); referenced as `.IsAdmin` and `$.IsAdmin` in `certificates.html` — consistent.
