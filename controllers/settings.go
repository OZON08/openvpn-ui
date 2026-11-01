package controllers

import (

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/server/web"
	"github.com/OZON08/openvpn-ui/models"
	"github.com/OZON08/openvpn-ui/state"
)

type SettingsController struct {
	BaseController
}

func (c *SettingsController) NestPrepare() {
	c.RequireAdmin()
	c.Data["breadcrumbs"] = &BreadCrumbs{
		Title: "OpenVPN UI Settings",
	}
}

func (c *SettingsController) Get() {
	c.TplName = "settings.html"
	settings := models.Settings{Profile: "default"}
	_ = settings.Read("Profile")
	c.Data["Settings"] = &settings
}

func (c *SettingsController) Post() {
	c.TplName = "settings.html"

	flash := web.NewFlash()
	settings := models.Settings{Profile: "default"}
	_ = settings.Read("Profile")
	if err := c.ParseForm(&settings); err != nil {
		logs.Warning(err)
		flash.Error("%s", err.Error())
		flash.Store(&c.Controller)
		return
	}
	c.Data["Settings"] = &settings

	o := orm.NewOrm()
	if _, err := o.Update(&settings); err != nil {
		flash.Error("%s", err.Error())
	} else {
		flash.Success("Settings has been updated")
		state.GlobalCfgMu.Lock()
		state.GlobalCfg = settings
		state.GlobalCfgMu.Unlock()
		models.WriteAuditLog(c.Userinfo.Id, c.Userinfo.Login, "settings.save", "", "", c.Ctx.Input.IP())
	}
	flash.Store(&c.Controller)
}
