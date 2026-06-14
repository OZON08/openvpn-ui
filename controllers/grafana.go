package controllers

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"

	"github.com/beego/beego/v2/server/web"
)

// GrafanaController reverse-proxies all /grafana/* requests to the internal
// Grafana instance. Authentication is enforced via openvpn-ui's session; the
// logged-in username is forwarded via X-WEBAUTH-USER so Grafana's auth proxy
// feature can sign the user in automatically.
type GrafanaController struct {
	BaseController
}

func (c *GrafanaController) Prepare() {
	c.EnableXSRF = false
	c.BaseController.Prepare()
}

func (c *GrafanaController) NestPrepare() {
	if !c.IsLogin {
		c.Ctx.Redirect(302, c.LoginPath())
		c.StopRun()
		return
	}
}

func (c *GrafanaController) Get() {
	c.TplName = "grafana.html"
	c.Data["breadcrumbs"] = &BreadCrumbs{Title: "Grafana"}
}

func (c *GrafanaController) Proxy() {
	if !c.IsLogin || c.Userinfo == nil {
		c.Ctx.Redirect(302, c.LoginPath())
		return
	}

	rawURL := web.AppConfig.DefaultString("GrafanaURL", os.Getenv("OPENVPN_UI_GRAFANA_URL"))
	if rawURL == "" {
		c.Ctx.Output.SetStatus(http.StatusNotFound)
		return
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		c.Ctx.Output.SetStatus(http.StatusBadGateway)
		return
	}

	username := c.Userinfo.Name

	originalHost := c.Ctx.Request.Host
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = originalHost
			req.Header.Set("X-WEBAUTH-USER", username)
			req.Header.Del("X-Forwarded-User")
		},
	}
	proxy.ServeHTTP(c.Ctx.ResponseWriter, c.Ctx.Request)
}
