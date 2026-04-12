package lib

import (
	"errors"
	"os"
	"os/exec"

	"github.com/beego/beego/v2/core/logs"
	"github.com/OZON08/openvpn-ui/state"
)

func DeletePKI(name string) error {
	if !SafeNameRegex.MatchString(name) {
		return errors.New("invalid name")
	}
	cmd := exec.Command("/bin/bash", "/opt/scripts/remove.sh", name)
	cmd.Dir = state.GlobalCfg.OVConfigPath
	cmd.Env = os.Environ()
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
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		logs.Debug(string(output))
		logs.Error(err)
		return err
	}
	return nil
}

func RestartContainer(name string) error {
	if !SafeNameRegex.MatchString(name) {
		return errors.New("invalid name")
	}
	cmd := exec.Command("/bin/bash", "/opt/scripts/restart.sh", name)
	cmd.Dir = state.GlobalCfg.OVConfigPath
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		logs.Debug(string(output))
		logs.Error(err)
		return err
	}
	return nil
}
