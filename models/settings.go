package models

import (
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/beego/beego/v2/client/orm"
	//Sqlite driver
	_ "github.com/mattn/go-sqlite3"
)

type Settings struct {
	Id      int64
	Profile string `orm:"size(64);unique" form:"Profile" valid:"Required;"`

	MIAddress string `orm:"size(64);unique" form:"MIAddress" valid:"Required;"`
	MINetwork string `orm:"size(64);unique" form:"MINetwork" valid:"Required;"`

	OVConfigPath string `orm:"size(64);unique" form:"OVConfigPath" valid:"Required;"`
	EasyRSAPath  string `orm:"size(64);unique" form:"EasyRSAPath" valid:"Required;"`

	//	ServerAddress     string `orm:"size(64);unique" form:"ServerAddress" valid:"Required;"`
	//	OpenVpnServerPort string `orm:"size(64);unique" form:"OpenVpnServerPort" valid:"Required;"`

	Created time.Time `orm:"auto_now_add;type(datetime)"`
	Updated time.Time `orm:"auto_now;type(datetime)"`
}

// socketPath matches paths that stay one argument of the management directive.
var socketPath = regexp.MustCompile(`^/[A-Za-z0-9._/-]+$`)

// ManagementDirective returns the arguments of the OpenVPN management directive for
// these settings. A TCP server binds to the stored IP address. Host names are rejected,
// because they would need a listener on all addresses, and the interface has no password.
func (s Settings) ManagementDirective() (string, error) {
	if s.MINetwork == "unix" && socketPath.MatchString(s.MIAddress) && filepath.Clean(s.MIAddress) == s.MIAddress {
		return s.MIAddress + " unix", nil
	}
	if strings.HasPrefix(s.MINetwork, "tcp") {
		if host, port, err := net.SplitHostPort(s.MIAddress); err == nil {
			if number, err := strconv.ParseUint(port, 10, 16); err == nil && number > 0 && net.ParseIP(host) != nil {
				return host + " " + port, nil
			}
		}
	}
	return "", fmt.Errorf("invalid management settings %q %q: use the socket path with unix, or an IP address and port with tcp", s.MIAddress, s.MINetwork)
}

// Insert wrapper
func (s *Settings) Insert() error {
	if _, err := orm.NewOrm().Insert(s); err != nil {
		return err
	}
	return nil
}

// Read wrapper
func (s *Settings) Read(fields ...string) error {
	if err := orm.NewOrm().Read(s, fields...); err != nil {
		return err
	}
	return nil
}

// Update wrapper
func (s *Settings) Update(fields ...string) error {
	if _, err := orm.NewOrm().Update(s, fields...); err != nil {
		return err
	}
	return nil
}

// Delete wrapper
func (s *Settings) Delete() error {
	if _, err := orm.NewOrm().Delete(s); err != nil {
		return err
	}
	return nil
}
