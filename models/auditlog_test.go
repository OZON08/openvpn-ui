package models

import (
	"os"
	"testing"
	"time"

	"github.com/beego/beego/v2/client/orm"
	_ "github.com/mattn/go-sqlite3"
)

func TestMain(m *testing.M) {
	_ = orm.RegisterDriver("sqlite3", orm.DRSqlite)
	_ = orm.RegisterDataBase("default", "sqlite3", ":memory:")
	orm.RegisterModel(new(AuditLog))
	if err := orm.RunSyncdb("default", false, false); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestWriteAuditLog(t *testing.T) {
	WriteAuditLog(1, "admin", "cert.create", "alice", "", "10.0.0.1")

	o := orm.NewOrm()
	var entries []*AuditLog
	if _, err := o.QueryTable("audit_log").Filter("Action", "cert.create").All(&entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one audit log entry")
	}
	e := entries[0]
	if e.UserID != 1 {
		t.Errorf("UserID: got %d, want 1", e.UserID)
	}
	if e.UserLogin != "admin" {
		t.Errorf("UserLogin: got %q, want %q", e.UserLogin, "admin")
	}
	if e.Action != "cert.create" {
		t.Errorf("Action: got %q, want %q", e.Action, "cert.create")
	}
	if e.Target != "alice" {
		t.Errorf("Target: got %q, want %q", e.Target, "alice")
	}
	if e.IPAddr != "10.0.0.1" {
		t.Errorf("IPAddr: got %q, want %q", e.IPAddr, "10.0.0.1")
	}
	if e.CreatedAt.IsZero() {
		t.Error("CreatedAt must not be zero")
	}
}

func TestPurgeOldAuditLogs(t *testing.T) {
	o := orm.NewOrm()
	// Insert an old entry directly via ORM, but with a modified timestamp afterward
	oldEntry := &AuditLog{
		UserID:    0,
		UserLogin: "system",
		Action:    "old.action",
		Target:    "",
		Detail:    "",
		IPAddr:    "",
	}
	if _, err := o.Insert(oldEntry); err != nil {
		t.Fatal("insert old entry:", err)
	}

	// Now update its timestamp to be old using raw SQL
	oldTime := time.Now().AddDate(0, 0, -400).Format("2006-01-02 15:04:05")
	if _, err := o.Raw(
		"UPDATE audit_log SET created_at = ? WHERE action = ?",
		oldTime, "old.action",
	).Exec(); err != nil {
		t.Fatal("update old entry timestamp:", err)
	}

	WriteAuditLog(1, "admin", "recent.action", "", "", "")

	if err := PurgeOldAuditLogs(365); err != nil {
		t.Fatal(err)
	}

	oldCount, _ := o.QueryTable("audit_log").Filter("Action", "old.action").Count()
	if oldCount != 0 {
		t.Errorf("expected old entry purged, got %d remaining", oldCount)
	}
	recentCount, _ := o.QueryTable("audit_log").Filter("Action", "recent.action").Count()
	if recentCount == 0 {
		t.Error("recent entry must not be purged")
	}
}
