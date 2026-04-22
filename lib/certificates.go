package lib

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/beego/beego/v2/core/logs"
	"github.com/OZON08/openvpn-ui/state"
)

// Cert
// https://groups.google.com/d/msg/mailing.openssl.users/gMRbePiuwV0/wTASgPhuPzkJ
type Cert struct {
	EntryType   string
	Expiration  string
	ExpirationT time.Time
	IsExpiring  bool
	Revocation  string
	RevocationT time.Time
	Serial      string
	FileName    string
	Details     *Details
}

type Details struct {
	Name             string
	CN               string
	Country          string
	State            string
	City             string
	Organisation     string
	OrganisationUnit string
	Email            string
	LocalIP          string
	TFAName          string
}

// Input validation regexes — allowlist approach
var (
	SafeNameRegex   = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	safeTextRegex   = regexp.MustCompile(`^[a-zA-Z0-9 .,_@+-]+$`)
	safeExpireRegex = regexp.MustCompile(`^[0-9]+$`)
)

// validateCertInputs checks all user-supplied parameters against allowlist regexes.
// This prevents command injection even if shell args are used.
func validateCertInputs(name, staticip, expiredays, email, country, province, city, org, orgunit, tfaname, tfaissuer string) error {
	if !SafeNameRegex.MatchString(name) {
		return errors.New("invalid name: only alphanumeric characters, dots, underscores and hyphens allowed")
	}
	if staticip != "" {
		if net.ParseIP(staticip) == nil {
			return fmt.Errorf("invalid static IP address: %q", staticip)
		}
	}
	if expiredays != "" && !safeExpireRegex.MatchString(expiredays) {
		return errors.New("invalid expire days: must be numeric")
	}
	fields := map[string]string{
		"email": email, "country": country, "province": province,
		"city": city, "org": org, "orgunit": orgunit,
		"tfaname": tfaname, "tfaissuer": tfaissuer,
	}
	for field, value := range fields {
		if value != "" && !safeTextRegex.MatchString(value) {
			return fmt.Errorf("invalid characters in field %q", field)
		}
	}
	return nil
}

// buildOpenVPNEnv returns the process environment plus EASY_RSA/OPENVPN_DIR
// derived from the current runtime config, with any additional caller-supplied
// variables appended. Shell scripts under /opt/scripts use these two vars
// instead of reading app.conf via a relative path (which breaks because
// cmd.Dir = /etc/openvpn, not /opt/openvpn).
func buildOpenVPNEnv(extras ...string) []string {
	env := append(os.Environ(),
		"EASY_RSA="+state.GlobalCfg.EasyRSAPath,
		"OPENVPN_DIR="+state.GlobalCfg.OVConfigPath,
	)
	return append(env, extras...)
}

// buildCertEnv returns the process environment extended with EasyRSA variables.
func buildCertEnv(name, tfaname, tfaissuer, expiredays, email, country, province, city, org, orgunit string) []string {
	return buildOpenVPNEnv(
		"KEY_NAME="+name,
		"TFA_NAME="+tfaname,
		"TFA_ISSUER="+tfaissuer,
		"EASYRSA_CERT_EXPIRE="+expiredays,
		"EASYRSA_REQ_EMAIL="+email,
		"EASYRSA_REQ_COUNTRY="+country,
		"EASYRSA_REQ_PROVINCE="+province,
		"EASYRSA_REQ_CITY="+city,
		"EASYRSA_REQ_ORG="+org,
		"EASYRSA_REQ_OU="+orgunit,
	)
}

func ReadCerts(path string) ([]*Cert, error) {
	certs := make([]*Cert, 0)
	text, err := os.ReadFile(path)
	if err != nil {
		return certs, err
	}
	lines := strings.Split(trim(string(text)), "\n")
	for _, line := range lines {
		fields := strings.Split(trim(line), "\t")
		if len(fields) != 6 {
			return certs,
				fmt.Errorf("incorrect number of lines in line: \n%s\n. Expected %d, found %d",
					line, 6, len(fields))
		}
		expT, _ := time.Parse("060102150405Z", fields[1])
		expTA := time.Now().AddDate(0, 0, 30).After(expT) // If cer will expire in 30 days, raise this flag
		revT, _ := time.Parse("060102150405Z", fields[2])
		c := &Cert{
			EntryType:   fields[0],
			Expiration:  fields[1],
			ExpirationT: expT,
			IsExpiring:  expTA,
			Revocation:  fields[2],
			RevocationT: revT,
			Serial:      fields[3],
			FileName:    fields[4],
			Details:     parseDetails(fields[5]),
		}
		certs = append(certs, c)
	}

	return certs, nil
}

func parseDetails(d string) *Details {
	details := &Details{}
	lines := strings.Split(trim(d), "/")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Split(trim(line), "=")
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "name":
			details.Name = fields[1]
		case "CN":
			details.CN = fields[1]
		case "C":
			details.Country = fields[1]
		case "ST":
			details.State = fields[1]
		case "L":
			details.City = fields[1]
		case "O":
			details.Organisation = fields[1]
		case "OU":
			details.OrganisationUnit = fields[1]
		case "emailAddress":
			details.Email = fields[1]
		case "LocalIP":
			details.LocalIP = fields[1]
		case "2FAName":
			details.TFAName = fields[1]
		default:
			if !strings.Contains(line, "name") && !strings.Contains(line, "LocalIP") {
				logs.Warn(fmt.Sprintf("Undefined entry: %s", line))
			}
		}
	}
	return details
}

func trim(s string) string {
	return strings.Trim(strings.Trim(s, "\r\n"), "\n")
}

func CreateCertificate(name string, staticip string, passphrase string, expiredays string, email string, country string, province string, city string, org string, orgunit string, tfaname string, tfaissuer string) error {
	logs.Info("Lib: Creating certificate: name=%s, staticip=%s, expiredays=%s", name, staticip, expiredays)

	if err := validateCertInputs(name, staticip, expiredays, email, country, province, city, org, orgunit, tfaname, tfaissuer); err != nil {
		return err
	}

	path := state.GlobalCfg.OVConfigPath + "/pki/index.txt"
	haveip := staticip != ""
	pass := passphrase != ""

	existsError := errors.New("Error! There is already a valid or invalid certificate for the name \"" + name + "\"")
	certs, err := ReadCerts(path)
	if err != nil {
		logs.Error(err)
	}
	for _, v := range certs {
		if v.Details.Name == name {
			return existsError
		}
	}
	Dump(certs)

	if !haveip {
		staticip = "dynamic.pool"
	}

	// Build script arguments — passed as separate args, not through shell string interpolation
	scriptArgs := []string{"/opt/scripts/genclient.sh", name, staticip}
	if pass {
		scriptArgs = append(scriptArgs, passphrase)
	}

	cmd := exec.Command("/bin/bash", scriptArgs...)
	cmd.Dir = state.GlobalCfg.OVConfigPath
	cmd.Env = buildCertEnv(name, tfaname, tfaissuer, expiredays, email, country, province, city, org, orgunit)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logs.Debug(string(output))
		logs.Error(err)
		return err
	}

	// Write static IP config directly (replaces the former shell echo/redirect)
	if haveip {
		configPath := filepath.Join("/etc/openvpn/staticclients", filepath.Base(name))
		content := fmt.Sprintf("ifconfig-push %s 255.255.255.0\n", staticip)
		if err := os.WriteFile(configPath, []byte(content), 0640); err != nil {
			return fmt.Errorf("failed to write static IP config: %w", err)
		}
	}

	return nil
}

func RevokeCertificate(name string, serial string, tfaname string) error {
	if !SafeNameRegex.MatchString(name) {
		return errors.New("invalid certificate name")
	}
	if !safeTextRegex.MatchString(serial) {
		return errors.New("invalid serial")
	}

	cmd := exec.Command("/bin/bash", "/opt/scripts/revoke.sh", name, serial)
	cmd.Dir = state.GlobalCfg.OVConfigPath
	cmd.Env = buildOpenVPNEnv(
		"KEY_NAME="+name,
		"TFA_NAME="+tfaname,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logs.Debug(string(output))
		logs.Error(err)
		return err
	}
	return nil
}

func Restart() error {
	cmd := exec.Command("/bin/bash", "/opt/scripts/restart.sh")
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

func BurnCertificate(CN string, serial string, tfaname string) error {
	logs.Info("Lib: Burning certificate: CN=%s, serial=%s", CN, serial)

	if !SafeNameRegex.MatchString(CN) {
		return errors.New("invalid certificate CN")
	}
	if !safeTextRegex.MatchString(serial) {
		return errors.New("invalid serial")
	}

	cmd := exec.Command("/bin/bash", "/opt/scripts/rmcert.sh", CN, serial)
	cmd.Dir = state.GlobalCfg.OVConfigPath
	cmd.Env = buildOpenVPNEnv(
		"TFA_NAME="+tfaname,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logs.Debug(string(output))
		logs.Error(err)
		return err
	}
	return nil
}

func RenewCertificate(name string, localip string, serial string, tfaname string) error {
	if !SafeNameRegex.MatchString(name) {
		return errors.New("invalid certificate name")
	}
	if !safeTextRegex.MatchString(serial) {
		return errors.New("invalid serial")
	}

	cmd := exec.Command("/bin/bash", "/opt/scripts/renew.sh", name, localip, serial)
	cmd.Dir = state.GlobalCfg.OVConfigPath
	cmd.Env = buildOpenVPNEnv(
		"KEY_NAME="+name,
		"TFA_NAME="+tfaname,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logs.Debug(string(output))
		logs.Error(err)
		return err
	}
	return nil
}
