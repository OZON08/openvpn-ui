package controllers

import (
	"github.com/beego/beego/v2/client/orm"
	"github.com/OZON08/openvpn-ui/models"
)

type AuditLogController struct {
	BaseController
}

func (c *AuditLogController) NestPrepare() {
	if !c.IsLogin {
		c.Ctx.Redirect(302, c.LoginPath())
		c.StopRun()
		return
	}
	if c.Userinfo == nil || !c.Userinfo.IsAdmin {
		c.Ctx.Redirect(302, c.URLFor("MainController.Get"))
		c.StopRun()
		return
	}
	c.Data["breadcrumbs"] = &BreadCrumbs{Title: "Audit Log"}
}

func (c *AuditLogController) Get() {
	const pageSize = 50
	page, _ := c.GetInt("page", 1)
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	o := orm.NewOrm()
	var entries []*models.AuditLog
	count, _ := o.QueryTable("audit_log").Count()
	_, _ = o.QueryTable("audit_log").OrderBy("-created_at").Limit(pageSize, offset).All(&entries)

	totalPages := int((count + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}

	c.Data["Entries"] = entries
	c.Data["Page"] = page
	c.Data["TotalPages"] = totalPages
	c.Data["HasPrev"] = page > 1
	c.Data["HasNext"] = page < totalPages
	c.Data["PrevPage"] = page - 1
	c.Data["NextPage"] = page + 1
	c.TplName = "auditlog.html"
}
