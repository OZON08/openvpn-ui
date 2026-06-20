package lib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// validateCertInputs
// ---------------------------------------------------------------------------

func TestValidateCertInputs_Valid(t *testing.T) {
	err := validateCertInputs(
		"myclient", "10.0.0.1", "365",
		"user@example.com", "DE", "Bayern", "Munich",
		"Acme Corp", "IT", "mytoken", "MyIssuer",
	)
	if err != nil {
		t.Errorf("expected no error for valid input, got: %v", err)
	}
}

func TestValidateCertInputs_EmptyOptionalFields(t *testing.T) {
	// Only name is required; all other fields may be empty
	err := validateCertInputs("client01", "", "", "", "", "", "", "", "", "", "")
	if err != nil {
		t.Errorf("expected no error for minimal input, got: %v", err)
	}
}

func TestValidateCertInputs_InvalidName(t *testing.T) {
	cases := []string{
		"client; rm -rf /",
		"client$(whoami)",
		"client`id`",
		"client|cat /etc/passwd",
		"../etc/passwd",
		"client name", // spaces not allowed in name
	}
	for _, name := range cases {
		err := validateCertInputs(name, "", "", "", "", "", "", "", "", "", "")
		if err == nil {
			t.Errorf("expected error for name %q, got nil", name)
		}
	}
}

func TestValidateCertInputs_InvalidIP(t *testing.T) {
	cases := []string{
		"999.999.999.999",
		"not-an-ip",
		"10.0.0.1; rm -rf /",
	}
	for _, ip := range cases {
		err := validateCertInputs("valid", ip, "", "", "", "", "", "", "", "", "")
		if err == nil {
			t.Errorf("expected error for IP %q, got nil", ip)
		}
	}
}

func TestValidateCertInputs_InvalidExpireDays(t *testing.T) {
	cases := []string{"abc", "30; rm -rf /", "-1"}
	for _, days := range cases {
		err := validateCertInputs("valid", "", days, "", "", "", "", "", "", "", "")
		if err == nil {
			t.Errorf("expected error for expiredays %q, got nil", days)
		}
	}
}

func TestValidateCertInputs_InvalidTextField(t *testing.T) {
	// Shell metacharacters must be rejected in text fields
	injection := "Acme$(id)"
	err := validateCertInputs("valid", "", "", "", "", "", "", injection, "", "", "")
	if err == nil {
		t.Errorf("expected error for org %q, got nil", injection)
	}
}

// ---------------------------------------------------------------------------
// parseDetails
// ---------------------------------------------------------------------------

func TestParseDetails_ValidDN(t *testing.T) {
	dn := "/name=alice/CN=alice/C=DE/ST=Bayern/L=Munich/O=Acme/OU=IT/emailAddress=alice@example.com"
	d := parseDetails(dn)

	if d.Name != "alice" {
		t.Errorf("Name: want %q, got %q", "alice", d.Name)
	}
	if d.CN != "alice" {
		t.Errorf("CN: want %q, got %q", "alice", d.CN)
	}
	if d.Country != "DE" {
		t.Errorf("Country: want %q, got %q", "DE", d.Country)
	}
	if d.City != "Munich" {
		t.Errorf("City: want %q, got %q", "Munich", d.City)
	}
	if d.Email != "alice@example.com" {
		t.Errorf("Email: want %q, got %q", "alice@example.com", d.Email)
	}
}

func TestParseDetails_EmptyLine(t *testing.T) {
	// Should not panic on empty input
	d := parseDetails("")
	if d == nil {
		t.Fatal("expected non-nil Details")
	}
}

func TestParseDetails_MalformedLine_NoEquals(t *testing.T) {
	// A line without '=' must be skipped without panic
	d := parseDetails("/noequalshere/name=bob")
	if d.Name != "bob" {
		t.Errorf("Name: want %q, got %q", "bob", d.Name)
	}
}

func TestParseDetails_MalformedLine_EmptyKey(t *testing.T) {
	// A leading slash produces an empty first token — must not panic
	d := parseDetails("//name=carol")
	if d.Name != "carol" {
		t.Errorf("Name: want %q, got %q", "carol", d.Name)
	}
}

// ---------------------------------------------------------------------------
// SafeNameRegex
// ---------------------------------------------------------------------------

func TestSafeNameRegex_AllowedChars(t *testing.T) {
	allowed := []string{"client01", "my-vpn", "user.name", "VPN_Client"}
	for _, s := range allowed {
		if !SafeNameRegex.MatchString(s) {
			t.Errorf("SafeNameRegex: expected %q to be allowed", s)
		}
	}
}

func TestSafeNameRegex_BlockedChars(t *testing.T) {
	blocked := []string{
		"client;id", "client$(x)", "test`cmd`",
		"a b", "a/b", "a\\b", "a&b", "a|b",
	}
	for _, s := range blocked {
		if SafeNameRegex.MatchString(s) {
			t.Errorf("SafeNameRegex: expected %q to be blocked", s)
		}
	}
}

// ---------------------------------------------------------------------------
// trim
// ---------------------------------------------------------------------------

func TestTrim(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"hello\n", "hello"},
		{"\r\nhello\r\n", "hello"},
		{"hello", "hello"},
		{"", ""},
	}
	for _, tc := range cases {
		got := trim(tc.input)
		if got != tc.want {
			t.Errorf("trim(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ReadCerts — error path
// ---------------------------------------------------------------------------

func TestReadCerts_NonExistentFile(t *testing.T) {
	certs, err := ReadCerts("/nonexistent/path/index.txt")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
	if len(certs) != 0 {
		t.Errorf("expected empty slice, got %d certs", len(certs))
	}
}

func TestReadCerts_MalformedLine(t *testing.T) {
	// Create a temp file with a line that has fewer than 6 tab-separated fields
	dir := t.TempDir()
	path := dir + "/index.txt"
	if err := writeFile(path, "V\t230101000000Z\t\n"); err != nil {
		t.Fatal(err)
	}
	_, err := ReadCerts(path)
	if err == nil {
		t.Error("expected error for malformed line, got nil")
	}
	if !strings.Contains(err.Error(), "incorrect number") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// writeFile is a small helper to write a string to a file in tests.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

// ---------------------------------------------------------------------------
// ReadCerts — happy path
// ---------------------------------------------------------------------------

func TestReadCerts_ValidFile(t *testing.T) {
	path := filepath.Join("testdata", "index-valid.txt")
	certs, err := ReadCerts(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(certs) != 2 {
		t.Fatalf("expected 2 certs, got %d", len(certs))
	}

	// Active certificate
	a := certs[0]
	if a.EntryType != "V" {
		t.Errorf("EntryType: want %q, got %q", "V", a.EntryType)
	}
	if a.ExpirationT.IsZero() {
		t.Error("ExpirationT must not be zero for valid cert")
	}
	if a.ExpirationT.Year() != 2030 {
		t.Errorf("ExpirationT.Year: want 2030, got %d", a.ExpirationT.Year())
	}
	if a.Details.CN != "alice" {
		t.Errorf("CN: want %q, got %q", "alice", a.Details.CN)
	}
	if a.Details.Name != "alice" {
		t.Errorf("Name: want %q, got %q", "alice", a.Details.Name)
	}

	// Revoked certificate
	r := certs[1]
	if r.EntryType != "R" {
		t.Errorf("EntryType: want %q, got %q", "R", r.EntryType)
	}
	if r.RevocationT.IsZero() {
		t.Error("RevocationT must not be zero for revoked cert")
	}
	if r.RevocationT.Year() != 2024 {
		t.Errorf("RevocationT.Year: want 2024, got %d", r.RevocationT.Year())
	}
	if r.Details.CN != "bob" {
		t.Errorf("CN: want %q, got %q", "bob", r.Details.CN)
	}
	// No /name= in bob's DN — Name must fall back to CN
	if r.Details.Name != "bob" {
		t.Errorf("Name (CN fallback): want %q, got %q", "bob", r.Details.Name)
	}
}

// ---------------------------------------------------------------------------
// parseDetails — missing field coverage
// ---------------------------------------------------------------------------

func TestParseDetails_LocalIPAndTFAName(t *testing.T) {
	dn := "/CN=carol/name=carol/LocalIP=10.8.0.5/2FAName=mytoken"
	d := parseDetails(dn)
	if d.LocalIP != "10.8.0.5" {
		t.Errorf("LocalIP: want %q, got %q", "10.8.0.5", d.LocalIP)
	}
	if d.TFAName != "mytoken" {
		t.Errorf("TFAName: want %q, got %q", "mytoken", d.TFAName)
	}
}

func TestParseDetails_NameFallbackToCN(t *testing.T) {
	dn := "/CN=charlie"
	d := parseDetails(dn)
	if d.Name != "charlie" {
		t.Errorf("Name (CN fallback): want %q, got %q", "charlie", d.Name)
	}
}
