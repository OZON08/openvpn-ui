package controllers

import (
	"bufio"
	"os"
	"strings"

	"github.com/beego/beego/v2/core/logs"
	"github.com/OZON08/openvpn-ui/models"
)

type LogsController struct {
	BaseController
}

func (c *LogsController) NestPrepare() {
	if !c.IsLogin {
		c.Ctx.Redirect(302, c.LoginPath())
		return
	}
}

func (c *LogsController) Get() {
	c.TplName = "logs.html"
	c.Data["breadcrumbs"] = &BreadCrumbs{
		Title: "Logs",
	}

	settings := models.Settings{Profile: "default"}
	settings.Read("Profile")

	if err := settings.Read("OVConfigPath"); err != nil {
		logs.Error(err)
		return
	}

	fName := settings.OVConfigPath + "/log/openvpn.log"
	file, err := os.Open(fName)
	if err != nil {
		logs.Error(err)
		return
	}
	defer file.Close()

	var allowedCerts []string
	if !c.Userinfo.IsAdmin {
		allowedCerts, _ = models.CertsForUser(c.Userinfo.Id)
	}

	scanner := bufio.NewScanner(file)
	var logLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, " MANAGEMENT: ") {
			continue
		}
		if !c.Userinfo.IsAdmin && !lineMatchesCert(line, allowedCerts) {
			continue
		}
		logLines = append(logLines, strings.Trim(line, "\t"))
	}
	start := len(logLines) - 300
	if start < 0 {
		start = 0
	}
	c.Data["logs"] = logLines[start:]
}

// lineMatchesCert reports whether a log line concerns one of the given cert names.
func lineMatchesCert(line string, names []string) bool {
	for _, n := range names {
		if strings.Contains(line, "CN="+n+",") ||
			strings.Contains(line, "CN="+n+" ") ||
			strings.HasSuffix(line, "CN="+n) ||
			strings.Contains(line, "["+n+"]") ||
			strings.HasPrefix(line, n+"/") ||
			strings.Contains(line, " "+n+"/") {
			return true
		}
	}
	return false
}

//func reverse(lines []string) []string {
//	for i := 0; i < len(lines)/2; i++ {
//		j := len(lines) - i - 1
//		lines[i], lines[j] = lines[j], lines[i]
//	}
//	return lines
//}
