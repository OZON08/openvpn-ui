package controllers

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"html/template"
	"net/http"
)

const csrfSessionKey = "csrf"

// validCSRFToken compares in constant time. An empty token is never valid.
func validCSRFToken(want, got string) bool {
	return want != "" && subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

func newCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// protectCSRF rejects forged requests and gives the views the token of this session.
// It replaces Beego's EnableXSRF, whose cookie is Secure and so fails on plain HTTP.
// Controllers that authenticate differently set EnableXSRF to false before Prepare.
func (c *BaseController) protectCSRF() {
	if !c.EnableXSRF {
		return
	}
	token, _ := c.GetSession(csrfSessionKey).(string)
	if !isSafeMethod(c.Ctx.Input.Method()) {
		got := c.GetString("_xsrf")
		if got == "" {
			got = c.Ctx.Input.Header("X-Xsrftoken")
		}
		if !validCSRFToken(token, got) {
			c.Ctx.Output.SetStatus(http.StatusForbidden)
			_ = c.Ctx.Output.Body([]byte(c.T("Invalid or missing CSRF token. Reload the page and try again.")))
			c.StopRun()
		}
	}
	if token == "" {
		token = newCSRFToken()
		if err := c.SetSession(csrfSessionKey, token); err != nil {
			panic(err)
		}
	}
	c.Data["xsrfToken"] = token
	c.Data["xsrfdata"] = template.HTML(`<input type="hidden" name="_xsrf" value="` + token + `">`)
}
