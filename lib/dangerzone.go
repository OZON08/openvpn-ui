package lib

import (
	"errors"
	"os"
	"os/exec"
	"time"

	mi "github.com/OZON08/openvpn-server-config/server/mi"

	"github.com/beego/beego/v2/core/logs"
	"github.com/OZON08/openvpn-ui/state"
)

// Container names of the restart actions.
const (
	ContainerServer = "openvpn-server"
	ContainerUI     = "openvpn-ui"
)

func DeletePKI(name string) error {
	if !SafeNameRegex.MatchString(name) {
		return errors.New("invalid name")
	}
	cmd := exec.Command("/bin/bash", "/opt/scripts/remove.sh", name)
	cmd.Dir = state.GlobalCfg.OVConfigPath
	cmd.Env = buildOpenVPNEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		logs.Debug(string(output))
		logs.Error(err)
		return err
	}
	return nil
}

func InitPKI(name string) error {
	if !SafeNameRegex.MatchString(name) {
		return errors.New("invalid name")
	}
	cmd := exec.Command("/bin/bash", "/opt/scripts/generate_ca_and_server_certs.sh", name)
	cmd.Dir = state.GlobalCfg.OVConfigPath
	cmd.Env = buildOpenVPNEnv()
	output, err := cmd.CombinedOutput()
	if err != nil {
		logs.Debug(string(output))
		logs.Error(err)
		return err
	}
	return nil
}

// RestartContainer stops the process of a container, which its restart policy then
// starts again. This needs no access to the Docker socket, which would give the web
// process control over the host. It requires "restart: always" on the services.
func RestartContainer(name string) error {
	switch name {
	case ContainerServer:
		return mi.NewClient(state.GlobalCfg.MINetwork, state.GlobalCfg.MIAddress).Signal("SIGTERM")
	case ContainerUI:
		// Wait until the redirect of this request is answered.
		time.AfterFunc(time.Second, func() { os.Exit(1) })
		return nil
	}
	return errors.New("invalid name")
}
