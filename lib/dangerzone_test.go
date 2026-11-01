package lib

import "testing"

func TestRestartContainerRejectsOtherNames(t *testing.T) {
	for _, name := range []string{"", "other", "openvpn", "../openvpn-ui"} {
		if err := RestartContainer(name); err == nil {
			t.Errorf("%q: expected an error", name)
		}
	}
}
