// Package routers defines application routes
// @APIVersion 1.0.0.0-dev
// @Title OpenVPN API
// @Description REST API allows you to control and monitor your OpenVPN server
// @Contact adam.walach@gmail.com
// License Apache 2.0
// LicenseUrl http://www.apache.org/licenses/LICENSE-2.0.html
package routers

import (
	"github.com/beego/beego/v2/server/web"
	"github.com/OZON08/openvpn-ui/controllers"
)

func Init(configDir string) {
	web.SetStaticPath("/api/docs", "swagger")
	web.Router("/", &controllers.MainController{})
	web.Router("/login", &controllers.LoginController{}, "get:Login;post:Login")
	web.Router("/logout", &controllers.LoginController{}, "get:Logout")
	web.Router("/auth/google", &controllers.LoginController{}, "get:GoogleLogin")
	web.Router("/auth/google/callback", &controllers.LoginController{}, "get:GoogleCallback")	
	web.Router("/profile", &controllers.ProfileController{})
	web.Router("/profile/cert/assign", &controllers.ProfileController{}, "post:AssignCert")
	web.Router("/profile/cert/remove", &controllers.ProfileController{}, "post:RemoveCert")
	web.Router("/profile/cert/transfer", &controllers.ProfileController{}, "post:TransferCert")
	web.Router("/profile/cert/seed", &controllers.ProfileController{}, "post:SeedCerts")
	web.Router("/settings", &controllers.SettingsController{})
	web.Router("/ov/config", &controllers.OVConfigController{})
	web.Router("/logs", &controllers.LogsController{})
	web.Router("/ov/clientconfig", &controllers.OVClientConfigController{ConfigDir: configDir})
	web.Router("/easyrsa/config", &controllers.EasyRSAConfigController{ConfigDir: configDir})
	web.Router("/dangerzone", &controllers.DangerController{})
	web.Router("/monitor", &controllers.MonitorController{})
	web.Router("/monitor/influx", &controllers.MonitorController{}, "post:SaveInflux")
	web.Router("/grafana", &controllers.GrafanaController{}, "get:Get;*:Proxy")
	web.Router("/grafana/*", &controllers.GrafanaController{}, "*:Proxy")
	web.Router("/api-docs", &controllers.APIDocsController{})

	web.Include(&controllers.CertificatesController{ConfigDir: configDir})
	web.Include(&controllers.DangerController{})
	web.Include(&controllers.OVConfigController{ConfigDir: configDir})
	web.Include(&controllers.OVClientConfigController{ConfigDir: configDir})
	web.Include(&controllers.ProfileController{})

	ns := web.NewNamespace("/api/v1",
		web.NSNamespace("/session",
			web.NSInclude(
				&controllers.APISessionController{},
			),
		),
		web.NSNamespace("/sysload",
			web.NSInclude(
				&controllers.APISysloadController{},
			),
		),
		web.NSNamespace("/signal",
			web.NSInclude(
				&controllers.APISignalController{},
			),
		),
		web.NSNamespace("/monitor",
			web.NSRouter("/sessions", &controllers.APIMonitorSessionsController{}, "get:Get"),
			web.NSRouter("/traffic", &controllers.APIMonitorTrafficController{}, "get:Get"),
			web.NSRouter("/disconnect", &controllers.APIMonitorHookController{}, "post:Post"),
			web.NSRouter("/retention", &controllers.APIMonitorRetentionController{}, "get:Get"),
			web.NSRouter("/influx", &controllers.APIMonitorInfluxController{}, "get:Get"),
		),
	)
	web.AddNamespace(ns)
}
