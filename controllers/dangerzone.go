package controllers

import (
	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/server/web"
	"github.com/OZON08/openvpn-ui/lib"
	"github.com/OZON08/openvpn-ui/models"
)

type DangerController struct {
	BaseController
}

func (c *DangerController) NestPrepare() {
	c.RequireAdmin()
}

func (c *DangerController) Get() {
	c.TplName = "maintenance.html"
	c.Data["breadcrumbs"] = &BreadCrumbs{
		Title: "Maintenance",
	}
}

// @router /pki/delete/:key [post]
func (c *DangerController) DeletePKI() {
	flash := web.NewFlash()
	name := c.GetString(":key")
	//logs.Info("Controller: Deleting:", name)
	if err := lib.DeletePKI(name); err != nil {
		logs.Error(err)
		flash.Error("%s", err.Error())
	} else {
		flash.Success(c.T("Success! The \"%s\" has been deleted"), name)
		models.WriteAuditLog(c.Userinfo.Id, c.Userinfo.Login, "pki.delete", name, "", c.Ctx.Input.IP())
	}
	c.redirectWithFlash(flash, c.URLFor("DangerController.Get"))
}

// @router /pki/init/:key [post]
func (c *DangerController) InitPKI() {
	flash := web.NewFlash()
	name := c.GetString(":key")
	//logs.Info("Controller: Runing init for:", name)
	if err := lib.InitPKI(name); err != nil {
		logs.Error(err)
		flash.Error("%s", err.Error())
	} else {
		flash.Success(c.T("Success! The \"%s\" has been initialized."), name)
	}
	c.redirectWithFlash(flash, c.URLFor("DangerController.Get"))
}

// @router /container/restart/:key [post]
func (c *DangerController) RestartContainer() {
	flash := web.NewFlash()
	name := c.GetString(":key")
	//logs.Info("Controller: Restarting:", name)
	if err := lib.RestartContainer(name); err != nil {
		logs.Error("Error restarting container:", err)
		//	logs.Error("Stack trace:", string(debug.Stack()))
		flash.Error("%s", err.Error())
	} else {
		flash.Success(c.T("Success! Container \"%s\" has been restarted"), name)
	}
	c.redirectWithFlash(flash, c.URLFor("DangerController.Get"))
}
