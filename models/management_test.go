package models

import "testing"

const (
	socketAddress = "/etc/openvpn/run/mi.sock"
	socketNetwork = "unix"
)

func TestMigrateLegacyManagementMovesOldDefaultToSocket(t *testing.T) {
	s := Settings{MIAddress: "openvpn:2080", MINetwork: "tcp"}

	if !migrateLegacyManagement(&s, socketAddress, socketNetwork) {
		t.Fatal("old default must be migrated")
	}
	if s.MIAddress != socketAddress || s.MINetwork != socketNetwork {
		t.Fatalf("unexpected settings: %s %s", s.MIAddress, s.MINetwork)
	}
}

func TestMigrateLegacyManagementKeepsCustomValues(t *testing.T) {
	for _, s := range []Settings{
		{MIAddress: "10.0.0.5:2080", MINetwork: "tcp"},
		{MIAddress: "openvpn:2080", MINetwork: "tcp4"},
		{MIAddress: socketAddress, MINetwork: socketNetwork},
	} {
		address, network := s.MIAddress, s.MINetwork
		if migrateLegacyManagement(&s, socketAddress, socketNetwork) {
			t.Errorf("%s %s must not be migrated", address, network)
		}
		if s.MIAddress != address || s.MINetwork != network {
			t.Errorf("%s %s was changed to %s %s", address, network, s.MIAddress, s.MINetwork)
		}
	}
}
