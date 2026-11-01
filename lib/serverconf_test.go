package lib

import (
	"strings"
	"testing"
)

const renderedConf = `management /etc/openvpn/run/mi.sock unix  # Set by OpenVPN UI
port 1195
tls-crypt pki/ta.key
push "route 10.0.60.0 255.255.255.0"     # Route to Home VPN subnet
#auth-user-pass-verify /opt/app/bin/checkpsw.sh via-file
up /opt/app/up.sh
# ncp-ciphers AES-256-GCM
`

const previousConf = `management /etc/openvpn/run/mi.sock unix  # Set by OpenVPN UI
port 1194
tls-auth pki/ta.key 0
push "route 10.0.50.0 255.255.255.0"     # Route to Home VPN subnet
down /opt/app/old-custom-option.sh
`

func TestHandLines(t *testing.T) {
	for name, tc := range map[string]struct{ current, want string }{
		"hook is kept":                 {previousConf + "client-disconnect /opt/app/bin/client-disconnect.sh\n", "client-disconnect /opt/app/bin/client-disconnect.sh"},
		"changed form value":           {previousConf, ""},
		"hand-edited form directive":   {"port 1300\n", ""},
		"management from the file":     {"management 0.0.0.0 2080\n", ""},
		"commented form field":         {"auth-user-pass-verify /x via-file\n", ""},
		"custom option in the form":    {"up /opt/app/up.sh\n", ""},
		"other value of a form option": {"up /opt/app/other.sh\n", ""},
		"cleared custom option":        {"down /opt/app/old-custom-option.sh\n", ""},
		"comment is dropped":           {"# client-disconnect /x\n; push \"a\"\n", ""},
		"second push is kept":          {"push \"dhcp-option DOMAIN example.invalid\"\n", "push \"dhcp-option DOMAIN example.invalid\""},
		"push of the form":             {"push  \"route 10.0.60.0 255.255.255.0\"   # text\n", ""},
		"former push of the form":      {"push \"route 10.0.50.0 255.255.255.0\"\n", ""},
		"windows line ending":          {"client-connect /x\r\n", "client-connect /x"},
		"inline block":                 {"<extra-certs>\nAAA\n</extra-certs>\n", "<extra-certs>\nAAA\n</extra-certs>"},
		"line is kept once":            {"mute 5\nmute 5\n", "mute 5"},
		"empty file":                   {"", ""},
	} {
		if got := HandLines(renderedConf, previousConf, tc.current); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}

func TestHandLinesSurviveTheNextSave(t *testing.T) {
	first := renderedConf + HandLinesHeader + "\nclient-disconnect /x\n"
	if got := HandLines(renderedConf, renderedConf, first); got != "client-disconnect /x" {
		t.Errorf("got %q", got)
	}
	if strings.Contains(HandLinesHeader, "\n") || !strings.HasPrefix(HandLinesHeader, "# ") {
		t.Errorf("the header must be one comment line: %q", HandLinesHeader)
	}
}
