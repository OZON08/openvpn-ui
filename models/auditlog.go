package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
)

type AuditLog struct {
	Id        int64     `orm:"auto"`
	CreatedAt time.Time `orm:"auto_now_add;type(datetime)"`
	UserID    int64
	UserLogin string `orm:"size(64)"`
	Action    string `orm:"size(32)"`
	Target    string `orm:"size(256)"`
	Detail    string `orm:"size(512)"`
	IPAddr    string `orm:"size(64)"`
}

func WriteAuditLog(userID int64, userLogin, action, target, detail, ip string) {
	o := orm.NewOrm()
	if _, err := o.Insert(&AuditLog{
		UserID:    userID,
		UserLogin: userLogin,
		Action:    action,
		Target:    target,
		Detail:    detail,
		IPAddr:    ip,
	}); err != nil {
		logs.Error("audit log write failed:", err)
	}
}

func PurgeOldAuditLogs(retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	_, err := orm.NewOrm().Raw("DELETE FROM audit_log WHERE created_at < ?", cutoff).Exec()
	return err
}

func RunAuditLogRetention(retentionDays int) {
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		if err := PurgeOldAuditLogs(retentionDays); err != nil {
			logs.Error("audit log retention cleanup failed:", err)
		}
	}
}
