package models

import (
	"testing"
	"time"
)

func TestFindActiveSessionSeparatesConnectionsOfOneName(t *testing.T) {
	at := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	first := &VpnSession{CommonName: "shared", RealIP: "203.0.113.10", ConnectedAt: at, Status: "active"}
	second := &VpnSession{CommonName: "shared", RealIP: "203.0.113.11", ConnectedAt: at, Status: "active"}
	for _, s := range []*VpnSession{first, second} {
		if err := s.Insert(); err != nil {
			t.Fatal(err)
		}
	}
	for ip, want := range map[string]int64{"203.0.113.10": first.Id, "203.0.113.11": second.Id} {
		got, err := FindActiveSession("shared", ip, at)
		if err != nil {
			t.Fatalf("%s: %v", ip, err)
		}
		if got.Id != want {
			t.Errorf("%s: got session %d, want %d", ip, got.Id, want)
		}
	}
	if _, err := FindActiveSession("shared", "203.0.113.99", at); err == nil {
		t.Error("expected no session for an unknown address")
	}
}
