package controllers

import "testing"

func TestIsAllowedSignal(t *testing.T) {
	for name, want := range map[string]bool{
		"SIGUSR1": true,
		"SIGUSR2": true,
		"SIGTERM": true,
		"SIGHUP":  false,
		"SIGKILL": false,
		"sigusr1": false,
		"":        false,
	} {
		if got := isAllowedSignal(name); got != want {
			t.Errorf("%q: got %v, want %v", name, got, want)
		}
	}
}
