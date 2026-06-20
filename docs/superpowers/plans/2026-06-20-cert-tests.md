# Cert Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining coverage gaps in `lib/certificates_test.go` — `ReadCerts` happy path (valid + revoked entries, field parsing, time parsing) and three missing `parseDetails` field cases (`LocalIP`, `TFAName`, Name-fallback-to-CN).

**Architecture:** One new fixture file (`lib/testdata/index-valid.txt`) and three new test functions appended to `lib/certificates_test.go`. No new production code. Tests run via `go test ./lib/...` with no external dependencies.

**Tech Stack:** Go 1.21+, standard library only (`path/filepath`, `testing`).

## Global Constraints

- Package declaration in test file stays `package lib` (not `package lib_test`) — required to access unexported `parseDetails` and `trim`
- No new exported symbols — test functions only
- Fixture file lives at `lib/testdata/index-valid.txt` (Go's standard testdata convention; tests run from the package directory so `filepath.Join("testdata", "index-valid.txt")` resolves correctly)
- Go binary: `/usr/local/go/bin/go` — run as `export PATH=$PATH:/usr/local/go/bin && go test ./lib/...`

---

### Task 1: ReadCerts Happy Path + parseDetails Gap Tests

**Files:**
- Create: `lib/testdata/index-valid.txt`
- Modify: `lib/certificates_test.go` (append three test functions, add `"path/filepath"` import)

**Interfaces:**
- Consumes: `ReadCerts(path string) ([]*Cert, error)` and `parseDetails(d string) *Details` — both already defined in `lib/certificates.go`
- Produces: nothing for later tasks (this is the only task)

---

- [ ] **Step 1: Create the fixture file**

Create `lib/testdata/index-valid.txt` with exactly the following content (fields separated by **tabs**; the revocation field of the valid cert is empty — two consecutive tabs between the expiry and the serial):

```
V	301215000000Z		A1B2C3D4	unknown	/CN=alice/name=alice/LocalIP=10.8.0.2/2FAName=none
R	291215000000Z	240601000000Z	A1B2C3D5	unknown	/CN=bob
```

Field breakdown (6 tab-separated fields per line):

| # | Valid cert | Revoked cert |
|---|---|---|
| 0 | `V` | `R` |
| 1 | `301215000000Z` (expiry: 2030-12-15) | `291215000000Z` (expiry: 2029-12-15) |
| 2 | *(empty)* | `240601000000Z` (revoked: 2024-06-01) |
| 3 | `A1B2C3D4` | `A1B2C3D5` |
| 4 | `unknown` | `unknown` |
| 5 | `/CN=alice/name=alice/LocalIP=10.8.0.2/2FAName=none` | `/CN=bob` |

Bob has no `/name=` field — his `Details.Name` must fall back to his CN.

- [ ] **Step 2: Add `"path/filepath"` to the import block**

In `lib/certificates_test.go`, update the import block from:

```go
import (
	"os"
	"strings"
	"testing"
)
```

to:

```go
import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)
```

- [ ] **Step 3: Write the three failing tests**

Append the following block to the end of `lib/certificates_test.go`:

```go
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
	if a.Details.CN != "alice" {
		t.Errorf("CN: want %q, got %q", "alice", a.Details.CN)
	}
	if a.Details.Name != "alice" {
		t.Errorf("Name: want %q, got %q", "alice", a.Details.Name)
	}
	if a.Details.LocalIP != "10.8.0.2" {
		t.Errorf("LocalIP: want %q, got %q", "10.8.0.2", a.Details.LocalIP)
	}
	if a.Details.TFAName != "none" {
		t.Errorf("TFAName: want %q, got %q", "none", a.Details.TFAName)
	}

	// Revoked certificate
	r := certs[1]
	if r.EntryType != "R" {
		t.Errorf("EntryType: want %q, got %q", "R", r.EntryType)
	}
	if r.RevocationT.IsZero() {
		t.Error("RevocationT must not be zero for revoked cert")
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
```

- [ ] **Step 4: Run tests — expect failure** (fixture file not yet present in Go test run — actually the fixture IS created in Step 1, so they should pass immediately; run anyway to confirm)

```bash
export PATH=$PATH:/usr/local/go/bin && go test ./lib/... -run "TestReadCerts_ValidFile|TestParseDetails_LocalIPAndTFAName|TestParseDetails_NameFallbackToCN" -v
```

Expected output:
```
--- PASS: TestReadCerts_ValidFile (0.00s)
--- PASS: TestParseDetails_LocalIPAndTFAName (0.00s)
--- PASS: TestParseDetails_NameFallbackToCN (0.00s)
PASS
ok  	github.com/OZON08/openvpn-ui/lib
```

If any test fails, fix the fixture or test logic before continuing.

- [ ] **Step 5: Run the full lib test suite — all must pass**

```bash
export PATH=$PATH:/usr/local/go/bin && go test ./lib/... -v
```

Expected: all existing tests plus the 3 new ones pass, no failures.

- [ ] **Step 6: Commit**

```bash
git add lib/testdata/index-valid.txt lib/certificates_test.go
git commit -m "test: add ReadCerts happy path and parseDetails field coverage"
```
