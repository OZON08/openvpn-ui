package controllers

import (
	"encoding/json"

	mi "github.com/OZON08/openvpn-server-config/server/mi"
	"github.com/OZON08/openvpn-ui/state"
)

// APISignalController sends signals to OpenVPN daemon
type APISignalController struct {
	APIBaseController
}

// KillParams contains CommonName of session to kill
type SignalParams struct {
	Sname string `json:"sname"`
}

// isAllowedSignal rejects SIGHUP in particular: OpenVPN exits on it once server.conf
// has mode 0600, because it runs as nobody and cannot read the file again.
func isAllowedSignal(name string) bool {
	return name == "SIGUSR1" || name == "SIGUSR2" || name == "SIGTERM"
}

// Send signal to OpenVPN daemon
// @Title Send signal
// @Description Sends signal to OpenVPN daemon
// @Param    body     body     controllers.SignalParams     true      "Signal to send"
// @Success 200 request success
// @Failure 400 request failure
// @router / [post]
func (c *APISignalController) Send() {
	if c.Userinfo == nil || !c.Userinfo.IsAdmin {
		c.ServeJSONError("You are not authorized")
		return
	}
	client := mi.NewClient(state.GlobalCfg.MINetwork, state.GlobalCfg.MIAddress)
	p := SignalParams{}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &p); err != nil {
		c.ServeJSONError(err.Error())
		return
	}
	if !isAllowedSignal(p.Sname) {
		c.ServeJSONError("unsupported signal: " + p.Sname)
		return
	}
	if err := client.Signal(p.Sname); err != nil {
		c.ServeJSONError(err.Error())
		return
	}

	c.ServeJSONMessage("Signal sent")
}
