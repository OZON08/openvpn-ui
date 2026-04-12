package controllers

import (
	"github.com/beego/beego/v2/server/web"
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
