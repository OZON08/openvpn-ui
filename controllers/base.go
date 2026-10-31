package controllers

import (
	"os"
	"slices"

	mi "github.com/OZON08/openvpn-server-config/server/mi"
	"github.com/beego/beego/v2/server/web"
	"github.com/OZON08/openvpn-ui/lib"
	"github.com/OZON08/openvpn-ui/models"
)

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

// visibleClients limits VPN sessions to those the logged-in user may see.
func (c *BaseController) visibleClients(clients []*mi.OVClient) []*mi.OVClient {
	if c.Userinfo != nil && c.Userinfo.IsAdmin {
		return clients
	}
	if c.Userinfo == nil {
		return filterClients(clients, nil)
	}
	allowed, err := models.CertsForUser(c.Userinfo.Id)
	if err != nil {
		return filterClients(clients, nil)
	}
	return filterClients(clients, allowed)
}

func filterClients(clients []*mi.OVClient, allowed []string) []*mi.OVClient {
	filtered := make([]*mi.OVClient, 0, len(clients))
	for _, client := range clients {
		if slices.Contains(allowed, client.CommonName) {
			filtered = append(filtered, client)
		}
	}
	return filtered
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
