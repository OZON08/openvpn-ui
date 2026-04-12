package controllers

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// generateOAuthState
// ---------------------------------------------------------------------------

func TestGenerateOAuthState_Length(t *testing.T) {
	state, err := generateOAuthState()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 16 random bytes → 32 hex characters
	if len(state) != 32 {
		t.Errorf("expected length 32, got %d", len(state))
	}
}

func TestGenerateOAuthState_IsHex(t *testing.T) {
	state, err := generateOAuthState()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	const hexChars = "0123456789abcdef"
	for _, c := range state {
		if !strings.ContainsRune(hexChars, c) {
			t.Errorf("non-hex character %q in state %q", c, state)
		}
	}
}

func TestGenerateOAuthState_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		state, err := generateOAuthState()
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if seen[state] {
			t.Errorf("duplicate state value generated: %q", state)
		}
		seen[state] = true
	}
}

func TestGenerateOAuthState_NotStatic(t *testing.T) {
	a, _ := generateOAuthState()
	b, _ := generateOAuthState()
	if a == b {
		t.Error("two consecutive calls returned the same state — not random")
	}
}

// ---------------------------------------------------------------------------
// Email domain extraction (inline logic from GoogleCallback)
// ---------------------------------------------------------------------------

func splitEmailDomain(email string) (string, bool) {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "", false
	}
	return parts[1], true
}

func TestEmailDomainSplit_Valid(t *testing.T) {
	domain, ok := splitEmailDomain("user@example.com")
	if !ok {
		t.Fatal("expected ok for valid email")
	}
	if domain != "example.com" {
		t.Errorf("expected %q, got %q", "example.com", domain)
	}
}

func TestEmailDomainSplit_NoAt(t *testing.T) {
	_, ok := splitEmailDomain("notanemail")
	if ok {
		t.Error("expected not-ok for email without @")
	}
}

func TestEmailDomainSplit_MultipleAt(t *testing.T) {
	// Addresses with two @ signs must be rejected
	_, ok := splitEmailDomain("user@allowed.com@evil.com")
	if ok {
		t.Error("expected not-ok for email with multiple @ signs")
	}
}

func TestEmailDomainSplit_Empty(t *testing.T) {
	_, ok := splitEmailDomain("")
	if ok {
		t.Error("expected not-ok for empty string")
	}
}
