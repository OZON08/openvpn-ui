package monitor

import (
	"path/filepath"
	"testing"
	"time"
)

func testdata(name string) string {
	return filepath.Join("testdata", name)
}

// ---------------------------------------------------------------------------
// ParseStatusLog — error paths
// ---------------------------------------------------------------------------

func TestParseStatusLog_FileNotFound(t *testing.T) {
	clients, err := ParseStatusLog("/nonexistent/openvpn-status.log")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
	if clients != nil {
		t.Errorf("expected nil slice on error, got %v", clients)
	}
}

func TestParseStatusLog_EmptyFile(t *testing.T) {
	clients, err := ParseStatusLog(testdata("empty.log"))
	if err != nil {
		t.Fatalf("expected no error for empty file, got: %v", err)
	}
	if len(clients) != 0 {
		t.Errorf("expected 0 clients, got %d", len(clients))
	}
}

// ---------------------------------------------------------------------------
// ParseStatusLog — v2 format
// ---------------------------------------------------------------------------

func TestParseStatusLog_V2(t *testing.T) {
	clients, err := ParseStatusLog(testdata("v2-basic.log"))
	if err != nil {
		t.Fatalf("ParseStatusLog: %v", err)
	}
	if len(clients) != 2 {
		t.Fatalf("expected 2 clients, got %d: %+v", len(clients), clients)
	}

	// Map iteration order is not guaranteed — look up by CommonName.
	byName := make(map[string]StatusClient, len(clients))
	for _, c := range clients {
		byName[c.CommonName] = c
	}

	alice, ok := byName["alice"]
	if !ok {
		t.Fatal("missing client 'alice'")
	}
	if alice.RealIP != "192.168.1.10" {
		t.Errorf("alice.RealIP: want %q, got %q", "192.168.1.10", alice.RealIP)
	}
	if alice.RealPort != "51234" {
		t.Errorf("alice.RealPort: want %q, got %q", "51234", alice.RealPort)
	}
	if alice.BytesReceived != 102400 {
		t.Errorf("alice.BytesReceived: want 102400, got %d", alice.BytesReceived)
	}
	if alice.BytesSent != 204800 {
		t.Errorf("alice.BytesSent: want 204800, got %d", alice.BytesSent)
	}
	if alice.VirtualIP != "10.8.0.2" {
		t.Errorf("alice.VirtualIP: want %q, got %q", "10.8.0.2", alice.VirtualIP)
	}
	wantAlice := time.Unix(1750416000, 0).UTC()
	if !alice.ConnectedAt.Equal(wantAlice) {
		t.Errorf("alice.ConnectedAt: want %v, got %v", wantAlice, alice.ConnectedAt)
	}

	bob, ok := byName["bob"]
	if !ok {
		t.Fatal("missing client 'bob'")
	}
	if bob.RealIP != "10.0.0.5" {
		t.Errorf("bob.RealIP: want %q, got %q", "10.0.0.5", bob.RealIP)
	}
	if bob.RealPort != "49123" {
		t.Errorf("bob.RealPort: want %q, got %q", "49123", bob.RealPort)
	}
	if bob.BytesReceived != 512 {
		t.Errorf("bob.BytesReceived: want 512, got %d", bob.BytesReceived)
	}
	if bob.BytesSent != 1024 {
		t.Errorf("bob.BytesSent: want 1024, got %d", bob.BytesSent)
	}
	if bob.VirtualIP != "10.8.0.3" {
		t.Errorf("bob.VirtualIP: want %q, got %q", "10.8.0.3", bob.VirtualIP)
	}
	wantBob := time.Unix(1750419600, 0).UTC()
	if !bob.ConnectedAt.Equal(wantBob) {
		t.Errorf("bob.ConnectedAt: want %v, got %v", wantBob, bob.ConnectedAt)
	}
}
