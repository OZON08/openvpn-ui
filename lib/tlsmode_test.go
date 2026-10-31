package lib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerTLSLine(t *testing.T) {
	for conf, want := range map[string]string{
		"port 1194\ntls-auth pki/ta.key 0\nverb 3\n":      "tls-auth pki/ta.key 0",
		"port 1194\r\ntls-crypt pki/ta.key\r\nverb 3\r\n": "tls-crypt pki/ta.key",
		"  tls-crypt pki/ta.key\n":                        "tls-crypt pki/ta.key",
		"# tls-auth pki/ta.key 0\ntls-crypt pki/ta.key\n": "tls-crypt pki/ta.key",
		"tls-crypt-v2 pki/server.key\n":                   "",
		"port 1194\n":                                     "",
	} {
		if got := ServerTLSLine(conf); got != want {
			t.Errorf("%q: got %q, want %q", conf, got, want)
		}
	}
}

func TestProfileTemplateForServer(t *testing.T) {
	const tpl = "client\n<tls-crypt>\n{{ .Ta }}\n</tls-crypt>\n"

	withAuth := ProfileTemplateForServer(tpl, "tls-auth pki/ta.key 0\n")
	if !strings.Contains(withAuth, "key-direction 1\n<tls-auth>\n{{ .Ta }}\n</tls-auth>") || strings.Contains(withAuth, "tls-crypt") {
		t.Errorf("tls-auth server: unexpected template:\n%s", withAuth)
	}
	for _, conf := range []string{"tls-crypt pki/ta.key\n", "port 1194\n"} {
		if got := ProfileTemplateForServer(tpl, conf); got != tpl {
			t.Errorf("%q: template must stay on tls-crypt:\n%s", conf, got)
		}
	}
}

func TestWritePrivateFileTightensExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.ovpn")
	if err := os.WriteFile(path, []byte("older and longer content"), 0660); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0660); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivateFile(path, "new"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("unexpected mode: %o", info.Mode().Perm())
	}
	if data, _ := os.ReadFile(path); string(data) != "new" {
		t.Errorf("unexpected content: %q", data)
	}
}
