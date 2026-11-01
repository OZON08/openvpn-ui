package controllers

import "testing"

func TestValidCSRFToken(t *testing.T) {
	for _, tc := range []struct {
		want, got string
		ok        bool
	}{
		{"abc", "abc", true},
		{"abc", "abd", false},
		{"abc", "", false},
		{"", "", false},
		{"", "abc", false},
	} {
		if got := validCSRFToken(tc.want, tc.got); got != tc.ok {
			t.Errorf("validCSRFToken(%q, %q) = %v, want %v", tc.want, tc.got, got, tc.ok)
		}
	}
}

func TestIsSafeMethod(t *testing.T) {
	for method, want := range map[string]bool{
		"GET": true, "HEAD": true, "OPTIONS": true,
		"POST": false, "PUT": false, "PATCH": false, "DELETE": false,
	} {
		if got := isSafeMethod(method); got != want {
			t.Errorf("isSafeMethod(%q) = %v, want %v", method, got, want)
		}
	}
}

func TestNewCSRFTokenIsUnique(t *testing.T) {
	first, second := newCSRFToken(), newCSRFToken()
	if len(first) != 64 || first == second {
		t.Errorf("tokens %q and %q are not 32 random bytes", first, second)
	}
}
