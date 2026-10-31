package models

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/OZON08/openvpn-server-config/server/config"
)

const socketNetwork = "unix"

// sharedSocketPath returns a socket path whose directory exists, like a mounted run directory.
func sharedSocketPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "mi.sock")
}

func TestMigrateLegacyManagementMovesPort2080ToSocket(t *testing.T) {
	socket := sharedSocketPath(t)
	for _, s := range []Settings{
		{MIAddress: "openvpn:2080", MINetwork: "tcp"},
		{MIAddress: "vpn:2080", MINetwork: "tcp4"},
		{MIAddress: "127.0.0.1:2080", MINetwork: "tcp"},
	} {
		if !migrateLegacyManagement(&s, socket, socketNetwork) {
			t.Fatalf("%s %s must be migrated", s.MIAddress, s.MINetwork)
		}
		if s.MIAddress != socket || s.MINetwork != socketNetwork {
			t.Fatalf("unexpected settings: %s %s", s.MIAddress, s.MINetwork)
		}
	}
}

func TestMigrateLegacyManagementKeepsOtherValues(t *testing.T) {
	socket := sharedSocketPath(t)
	for _, s := range []Settings{
		{MIAddress: "10.0.0.5:2081", MINetwork: "tcp"},
		{MIAddress: "not-an-address", MINetwork: "tcp"},
		{MIAddress: socket, MINetwork: socketNetwork},
	} {
		address, network := s.MIAddress, s.MINetwork
		if migrateLegacyManagement(&s, socket, socketNetwork) {
			t.Errorf("%s %s must not be migrated", address, network)
		}
		if s.MIAddress != address || s.MINetwork != network {
			t.Errorf("%s %s was changed to %s %s", address, network, s.MIAddress, s.MINetwork)
		}
	}
}

// Without the shared run directory the server stays on TCP, so the UI must stay there too.
func TestMigrateLegacyManagementWaitsForSharedDirectory(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "run", "mi.sock")
	s := Settings{MIAddress: "openvpn:2080", MINetwork: "tcp"}

	if migrateLegacyManagement(&s, missing, socketNetwork) {
		t.Fatal("must not migrate while the socket directory is missing")
	}
	if s.MIAddress != "openvpn:2080" || s.MINetwork != "tcp" {
		t.Fatalf("settings were changed to %s %s", s.MIAddress, s.MINetwork)
	}
}

func TestManagementDirective(t *testing.T) {
	for _, tc := range []struct{ address, network, want string }{
		{"/etc/openvpn/run/mi.sock", "unix", "/etc/openvpn/run/mi.sock unix"},
		{"10.0.0.5:2081", "tcp4", "10.0.0.5 2081"},
		{"127.0.0.1:2080", "tcp", "127.0.0.1 2080"},
		{"[::1]:2080", "tcp6", "::1 2080"},
	} {
		s := Settings{MIAddress: tc.address, MINetwork: tc.network}
		got, err := s.ManagementDirective()
		if err != nil {
			t.Errorf("%s %s: %v", tc.address, tc.network, err)
		}
		if got != tc.want {
			t.Errorf("%s %s: got %q, want %q", tc.address, tc.network, got, tc.want)
		}
	}
}

func TestManagementDirectiveRejectsUnusableSettings(t *testing.T) {
	for _, s := range []Settings{
		{MIAddress: "no-port", MINetwork: "tcp"},
		{MIAddress: "openvpn:2080", MINetwork: "tcp"},
		{MIAddress: "10.0.0.5:99999", MINetwork: "tcp"},
		{MIAddress: "10.0.0.5:0", MINetwork: "tcp"},
		{MIAddress: "/run/mi.sock pwfile", MINetwork: "unix"},
		{MIAddress: "/run/mi.sock#x", MINetwork: "unix"},
		{MIAddress: "/run/../tmp/mi.sock", MINetwork: "unix"},
		{MIAddress: "relative/mi.sock", MINetwork: "unix"},
		{MIAddress: "openvpn:2080", MINetwork: "udp"},
		{MIAddress: "", MINetwork: "unix"},
	} {
		if got, err := s.ManagementDirective(); err == nil {
			t.Errorf("%q %q: expected an error, got %q", s.MIAddress, s.MINetwork, got)
		}
	}
}

func TestServerTemplateRendersManagementSetting(t *testing.T) {
	tpl, err := os.ReadFile("../conf/openvpn-server-config.tpl")
	if err != nil {
		t.Fatal(err)
	}
	c := config.New()
	c.Management = "/custom/run/mi.sock unix"

	text, err := config.GetText(string(tpl), c)
	if err != nil {
		t.Fatal(err)
	}
	first, _, _ := strings.Cut(text, "\n")
	if !strings.HasPrefix(first, "management /custom/run/mi.sock unix") {
		t.Fatalf("unexpected management line: %q", first)
	}
}
