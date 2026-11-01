package controllers

import (
	"net/http"
	"os"
	"slices"
	"strings"

	mi "github.com/OZON08/openvpn-server-config/server/mi"
	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/server/web"
	"github.com/OZON08/openvpn-ui/lib"
	"github.com/OZON08/openvpn-ui/models"
)

const flashSessionKey = "flash"

type BaseController struct {
	web.Controller

	Userinfo *models.User
	IsLogin  bool
}

type NestPreparer interface {
	NestPrepare()
}

type NestFinisher interface {
	NestFinish()
}

func (c *BaseController) Prepare() {
	c.protectCSRF()
	c.showRedirectFlash()
	c.SetParams()

	userID := c.GetSession("userinfo")
	if userID != nil {
		var user models.User
		user.Id = userID.(int64)
		err := user.Read("Id")
		if err == nil {
			c.IsLogin = true
			c.Userinfo = &user
		} else {
			c.IsLogin = false
			c.DelSession("userinfo")
		}
	} else {
		c.IsLogin = false
	}

	c.Data["IsLogin"] = c.IsLogin
	c.Data["Userinfo"] = c.Userinfo
	c.Data["AppVersion"] = lib.Version
	grafanaURL := web.AppConfig.DefaultString("GrafanaURL", os.Getenv("OPENVPN_UI_GRAFANA_URL"))
	c.Data["GrafanaEnabled"] = grafanaURL != ""

	if app, ok := c.AppController.(NestPreparer); ok {
		app.NestPrepare()
	}
}

func (c *BaseController) Finish() {
	if app, ok := c.AppController.(NestFinisher); ok {
		app.NestFinish()
	}
}

func (c *BaseController) GetLogin() *models.User {
	id, ok := c.GetSession("userinfo").(int64)
	if !ok {
		return nil
	}
	u := &models.User{Id: id}
	if err := u.Read("Id"); err != nil {
		return nil
	}
	return u
}

func (c *BaseController) DelLogin() {
	c.DelSession("userinfo")
	c.IsLogin = false
	c.Userinfo = nil
}

func (c *BaseController) SetLogin(user *models.User) {
	c.SetSession("userinfo", user.Id)
	c.IsLogin = true
	c.Userinfo = user
}

// RequireAdmin ends the request unless an administrator is logged in.
func (c *BaseController) RequireAdmin() {
	if !c.IsLogin {
		c.Ctx.Redirect(302, c.LoginPath())
		c.StopRun()
	}
	if c.Userinfo == nil || !c.Userinfo.IsAdmin {
		c.Ctx.Redirect(302, c.URLFor("MainController.Get"))
		c.StopRun()
	}
}

// redirectWithFlash shows the message on the next page, so a reload cannot repeat the action.
func (c *BaseController) redirectWithFlash(flash *web.FlashData, target string) {
	if err := c.SetSession(flashSessionKey, flash.Data); err != nil {
		logs.Error(err)
	}
	c.Redirect(target, http.StatusSeeOther)
}

func (c *BaseController) showRedirectFlash() {
	if strings.HasPrefix(c.Ctx.Input.URL(), "/api/") {
		return
	}
	if data, ok := c.GetSession(flashSessionKey).(map[string]string); ok {
		c.Data["flash"] = data
		if err := c.DelSession(flashSessionKey); err != nil {
			logs.Error(err)
		}
	}
}

// canAccessCert reports whether the logged-in user may act on this certificate.
func (c *BaseController) canAccessCert(name string) bool {
	if c.Userinfo == nil {
		return false
	}
	if c.Userinfo.IsAdmin {
		return true
	}
	allowed, err := models.CertsForUser(c.Userinfo.Id)
	return err == nil && slices.Contains(allowed, name)
}

// scopeStatus limits a VPN status to the sessions the logged-in user may see.
func (c *BaseController) scopeStatus(status *mi.Status) {
	if c.Userinfo != nil && c.Userinfo.IsAdmin {
		return
	}
	var allowed []string
	if c.Userinfo != nil {
		if names, err := models.CertsForUser(c.Userinfo.Id); err == nil {
			allowed = names
		}
	}
	limitStatus(status, allowed)
}

func limitStatus(status *mi.Status, allowed []string) {
	status.ClientList = slices.DeleteFunc(status.ClientList, func(client *mi.OVClient) bool {
		return !slices.Contains(allowed, client.CommonName)
	})
	status.RoutingTable = slices.DeleteFunc(status.RoutingTable, func(route *mi.RoutingPath) bool {
		return !slices.Contains(allowed, route.CommonName)
	})
}

func (c *BaseController) LoginPath() string {
	return c.URLFor("LoginController.Login")
}

func (c *BaseController) SetParams() {
	c.Data["Params"] = make(map[string]string)
	input, err := c.Input()
	if err != nil {
		// handle the error
		// log.Println("Error getting input:", err)
		return
	}
	for k, v := range input {
		if len(v) > 0 {
			c.Data["Params"].(map[string]string)[k] = v[0]
		}
	}
}

type BreadCrumbs struct {
	Title    string
	Subtitle string
}
