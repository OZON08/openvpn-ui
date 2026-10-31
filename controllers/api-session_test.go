package controllers

import (
	"testing"

	mi "github.com/OZON08/openvpn-server-config/server/mi"
)

func TestFilterClientsKeepsOnlyAllowedNames(t *testing.T) {
	clients := []*mi.OVClient{{CommonName: "mine"}, {CommonName: "other"}}

	got := filterClients(clients, []string{"mine"})

	if len(got) != 1 || got[0].CommonName != "mine" {
		t.Fatalf("unexpected clients: %d", len(got))
	}
	if got := filterClients(clients, nil); len(got) != 0 {
		t.Fatalf("expected no clients without assignments, got %d", len(got))
	}
}
