package controllers

import (
	"testing"

	mi "github.com/OZON08/openvpn-server-config/server/mi"
)

func TestLimitStatusKeepsOnlyAllowedNames(t *testing.T) {
	status := &mi.Status{
		ClientList:   []*mi.OVClient{{CommonName: "mine"}, {CommonName: "other"}},
		RoutingTable: []*mi.RoutingPath{{CommonName: "mine"}, {CommonName: "other"}},
	}

	limitStatus(status, []string{"mine"})

	if len(status.ClientList) != 1 || status.ClientList[0].CommonName != "mine" {
		t.Fatalf("unexpected clients: %d", len(status.ClientList))
	}
	if len(status.RoutingTable) != 1 || status.RoutingTable[0].CommonName != "mine" {
		t.Fatalf("unexpected routes: %d", len(status.RoutingTable))
	}

	limitStatus(status, nil)

	if len(status.ClientList) != 0 || len(status.RoutingTable) != 0 {
		t.Fatalf("expected nothing without assignments: %d clients, %d routes", len(status.ClientList), len(status.RoutingTable))
	}
}
