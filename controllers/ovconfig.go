package controllers

import (
	"os"
	"path/filepath"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/server/web"
	"github.com/OZON08/openvpn-server-config/server/config"
	mi "github.com/OZON08/openvpn-server-config/server/mi"
	"github.com/OZON08/openvpn-ui/lib"
	"github.com/OZON08/openvpn-ui/models"
	"github.com/OZON08/openvpn-ui/state"
)

type OVConfigController struct {
	BaseController
	ConfigDir string
}

func (c *OVConfigController) NestPrepare() {
	c.RequireAdmin()
	c.Data["breadcrumbs"] = &BreadCrumbs{
		Title: "OpenVPN Server configuration",
	}
}

// @router /ov/config [Get]
func (c *OVConfigController) Get() {
	c.TplName = "ovconfig.html"
	besettings := models.Settings{Profile: "default"}
	_ = besettings.Read("Profile")
	c.Data["BeeSettings"] = &besettings

	destPath := filepath.Join(state.GlobalCfg.OVConfigPath, "server.conf")
	serverConf, err := os.ReadFile(destPath)
	if err != nil {
		logs.Error(err)
		return
	}
	c.Data["ServerConfig"] = string(serverConf)
	cfg := models.OVConfig{Profile: "default"}
	_ = cfg.Read("Profile")
	// The file is what OpenVPN uses, and saving must not change the mode by accident
	if line := lib.ServerTLSLine(string(serverConf)); line != "" {
		cfg.TLSControlChannel = line
	}
	c.Data["Settings"] = &cfg

}

// @router /ov/config [Post]
func (c *OVConfigController) Post() {
	logs.Info("Starting Post method in OVConfigController")

	c.TplName = "ovconfig.html"
	flash := web.NewFlash()
	cfg := models.OVConfig{Profile: "default"}
	_ = cfg.Read("Profile")

	logs.Info("Post: Parsing form data")
	logs.Info("Form data before parsing: %v", c.Ctx.Request.Form)
	if err := c.ParseForm(&cfg); err != nil {
		logs.Warning(err)
		flash.Error("%s", err.Error())
		flash.Store(&c.Controller)
		return
	}
	logs.Info("Form data after parsing: %v", c.Ctx.Request.Form)

	logs.Info("Post: Dumping configuration data")
	logs.Info("Configuration data: %v", cfg)
	lib.Dump(cfg)
	c.Data["Settings"] = &cfg
	logs.Info("Settings data: %v", c.Data["Settings"])

	destPath := filepath.Join(state.GlobalCfg.OVConfigPath, "server.conf")
	logs.Info("Post: Saving configuration to file according to template")
	management, err := state.GlobalCfg.ManagementDirective()
	if err == nil {
		cfg.Management = management
		err = config.SaveToFile(filepath.Join(c.ConfigDir, "openvpn-server-config.tpl"), cfg.Config, destPath)
	}
	if err != nil {
		logs.Warning(err)
		flash.Error("%s", err.Error())
		flash.Store(&c.Controller)
		return
	}

	logs.Info("Post: Updating configuration in database")
	o := orm.NewOrm()
	if _, err := o.Update(&cfg); err != nil {
		flash.Error("%s", err.Error())
	} else {
		flash.Success("Post: Config has been updated")
		client := mi.NewClient(state.GlobalCfg.MINetwork, state.GlobalCfg.MIAddress)
		if err := client.Signal("SIGTERM"); err != nil {
			flash.Warning("Config has been updated but OpenVPN server was NOT reloaded: %s", err.Error())
		}
		models.WriteAuditLog(c.Userinfo.Id, c.Userinfo.Login, "ovconfig.save", "", "", c.Ctx.Input.IP())
	}

	logs.Info("Post: Reading updated server configuration from file")
	serverConf, err := os.ReadFile(destPath)
	if err != nil {
		logs.Error("Error reading server config from file:", err)
		flash.Error("Error reading server config from file")
		return
	}
	c.Data["ServerConfig"] = string(serverConf)

	flash.Store(&c.Controller)
}

// @router /ov/config/edit [Edit]
func (c *OVConfigController) Edit() {
	c.TplName = "ovconfig.html"
	flash := web.NewFlash()
	cfg := models.OVConfig{Profile: "default"}
	_ = cfg.Read("Profile")

	//logs.Info("Post: Parsing form data")
	if err := c.ParseForm(&cfg); err != nil {
		logs.Warning(err)
		flash.Error("%s", err.Error())
		flash.Store(&c.Controller)
		return
	}

	//logs.Info("Post: Dumping configuration data")
	lib.Dump(cfg)
	c.Data["Settings"] = &cfg

	//logs.Info("Starting Edit method in OVConfigController")
	destPath := filepath.Join(state.GlobalCfg.OVConfigPath, "server.conf")

	err := lib.ConfSaveToFile(destPath, c.GetString("ServerConfig"))
	if err != nil {
		logs.Error("Error saving server config to file:", err)
		flash.Error("Error saving server config to file")
		return
	} else {
		//logs.Info("Edit: Server config saved to file:", destPath)
		flash.Success("Config has been updated")
	}

	serverConf, err := os.ReadFile(destPath)
	if err != nil {
		logs.Error("Error reading server config from file:", err)
		flash.Error("Error reading server config from file")
		return
	}
	c.Data["ServerConfig"] = string(serverConf)

	flash.Store(&c.Controller)
}
