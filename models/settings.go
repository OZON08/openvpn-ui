package models

import (
	"fmt"
	"net"
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

// ManagementDirective returns the arguments of the OpenVPN management directive for
// these settings. A TCP server listens on all addresses, because the stored host is
// the name the UI connects to.
func (s Settings) ManagementDirective() (string, error) {
	if s.MINetwork == "unix" && s.MIAddress != "" {
		return s.MIAddress + " unix", nil
	}
	if strings.HasPrefix(s.MINetwork, "tcp") {
		if _, port, err := net.SplitHostPort(s.MIAddress); err == nil && port != "" {
			return "0.0.0.0 " + port, nil
		}
	}
	return "", fmt.Errorf("invalid management settings: %q %q", s.MIAddress, s.MINetwork)
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
