package main

import (
	"flag"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/server/web"
	"github.com/OZON08/openvpn-ui/lib"
	"github.com/OZON08/openvpn-ui/lib/monitor"
	"github.com/OZON08/openvpn-ui/models"
	"github.com/OZON08/openvpn-ui/routers"
	"github.com/OZON08/openvpn-ui/state"
)

func main() {
	configDir := flag.String("config", "conf", "Path to config dir")
	flag.Parse()

	configFile := filepath.Join(*configDir, "app.conf")
	fmt.Println("Config file:", configFile)

	if err := web.LoadAppConfig("ini", configFile); err != nil {
		panic(err)
	}

	// Debug output holds user records and full configurations.
	if web.BConfig.RunMode == web.PROD {
		logs.SetLevel(logs.LevelInformational)
	}

	models.InitDB()
	models.CreateDefaultUsers()
	defaultSettings, err := models.CreateDefaultSettings()
	if err != nil {
		panic(err)
	}

	retentionDays := 365
	if v, e := web.AppConfig.Int("AuditLogRetentionDays"); e == nil && v > 0 {
		retentionDays = v
	}
	go models.RunAuditLogRetention(retentionDays)

	models.CreateDefaultOVConfig(*configDir, defaultSettings.OVConfigPath, defaultSettings.MIAddress, defaultSettings.MINetwork)
	models.CreateDefaultOVClientConfig(*configDir, defaultSettings.OVConfigPath, defaultSettings.MIAddress, defaultSettings.MINetwork)
	models.CreateDefaultEasyRSAConfig(*configDir, defaultSettings.EasyRSAPath, defaultSettings.MIAddress, defaultSettings.MINetwork)
	state.GlobalCfg = *defaultSettings

	// Browsers leave the session cookie out of forged cross-site POST requests.
	web.BConfig.WebConfig.Session.SessionCookieSameSite = http.SameSiteLaxMode

	if err := lib.LoadTranslations(filepath.Join(*configDir, "locale")); err != nil {
		logs.Warn("Translations not loaded, using English: %v", err)
	}

	routers.Init(*configDir)

	lib.AddFuncMaps()

	mon, err := monitor.Start()
	if err != nil {
		logs.Warn("Monitor failed to start: %v", err)
	} else {
		state.Monitor = mon
	}

	web.Run()
}
