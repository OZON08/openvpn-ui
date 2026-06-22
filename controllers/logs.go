package controllers

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	"github.com/beego/beego/v2/core/logs"
	"github.com/OZON08/openvpn-ui/models"
)

var logCNRegex = regexp.MustCompile(`\bCN=([^,\s\]]+)`)

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

	ovc := models.OVConfig{Profile: "default"}
	if err := ovc.Read("Profile"); err != nil {
		logs.Error(err)
		c.Data["LogError"] = "Cannot read OpenVPN config from DB: " + err.Error()
		return
	}

	fName := ovc.OVConfigLogfile
	if fName == "" {
		c.Data["LogError"] = "Log file path is not configured (check OpenVPN Server settings → OVConfigLogfile)"
		return
	}

	file, err := os.Open(fName)
	if err != nil {
		logs.Error(err)
		c.Data["LogError"] = "Cannot open " + fName + ": " + err.Error()
		return
	}
	defer file.Close()

	var allowedSet map[string]bool
	if !c.Userinfo.IsAdmin {
		certs, _ := models.CertsForUser(c.Userinfo.Id)
		allowedSet = make(map[string]bool, len(certs))
		for _, n := range certs {
			if n != "" {
				allowedSet[n] = true
			}
		}
	}

	scanner := bufio.NewScanner(file)
	var logLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, " MANAGEMENT: ") {
			continue
		}
		if !c.Userinfo.IsAdmin && !lineMatchesCert(line, allowedSet) {
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

// lineMatchesCert reports whether a log line concerns one of the allowed cert names.
// Uses a regex to extract the CN field precisely; [name] and name/ patterns are
// inherently bounded. Empty allowed map → no match.
func lineMatchesCert(line string, allowed map[string]bool) bool {
	if len(allowed) == 0 {
		return false
	}
	if m := logCNRegex.FindStringSubmatch(line); m != nil && allowed[m[1]] {
		return true
	}
	for n := range allowed {
		if strings.Contains(line, "["+n+"]") {
			return true
		}
		if strings.HasPrefix(line, n+"/") || strings.Contains(line, " "+n+"/") {
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
