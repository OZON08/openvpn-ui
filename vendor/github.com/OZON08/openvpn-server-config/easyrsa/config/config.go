package easyrsaconfig

import (
	"fmt"
	"regexp"

	"github.com/OZON08/openvpn-server-config/internal/render"
)

// safeValue allows what organization names and addresses need. Easy-RSA sources
// the vars file as shell and expands values again in eval, so quoting cannot
// make other characters safe.
var safeValue = regexp.MustCompile(`^[\p{L}\p{M}\p{N} .,_@&+():-]*$`)

// Don't think these defaults are ever used -- see models/models.go
var defaultConfig = Config{
	EasyRSADN:          "org",
	EasyRSAReqCountry:  "UA",
	EasyRSAReqProvince: "KY",
	EasyRSAReqCity:     "Kyiv",
	EasyRSAReqOrg:      "SweetHome",
	EasyRSAReqEmail:    "sweet@home.net",
	EasyRSAReqOu:       "MyOrganizationalUnit",
	EasyRSAReqCn:       "server",
	EasyRSAKeySize:     2048,
	EasyRSACaExpire:    3650,
	EasyRSACertExpire:  825,
	EasyRSACertRenew:   30,
	EasyRSACrlDays:     180,
}

// Config model
type Config struct {
	EasyRSADN          string
	EasyRSAReqCountry  string
	EasyRSAReqProvince string
	EasyRSAReqCity     string
	EasyRSAReqOrg      string
	EasyRSAReqEmail    string
	EasyRSAReqOu       string
	EasyRSAReqCn       string

	EasyRSAKeySize    int
	EasyRSACaExpire   int
	EasyRSACertExpire int
	EasyRSACertRenew  int
	EasyRSACrlDays    int
}

// New returns config object with default values
func New() Config {
	return defaultConfig
}

// Validate rejects values that could run commands when Easy-RSA sources the vars file
func (c Config) Validate() error {
	return render.CheckStrings(c, func(name, value string) error {
		if !safeValue.MatchString(value) {
			return fmt.Errorf("invalid value in %s: only letters, digits, spaces and . , _ @ & + ( ) : - are allowed", name)
		}
		return nil
	})
}

// GetText validates config and injects its values into template
func GetText(tpl string, c Config) (string, error) {
	return render.Text(tpl, c)
}

// SaveToFile validates config, reads template and writes result to destination file
func SaveToFile(tplPath string, c Config, destPath string) error {
	return render.SaveToFile(tplPath, c, destPath)
}
