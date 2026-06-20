# StatusLog Parser Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add test coverage for `lib/monitor/statuslog.go` — the OpenVPN status file parser — using fixture files and package-internal table-driven tests.

**Architecture:** One new test file (`lib/monitor/statuslog_test.go`) in `package monitor` (not `package monitor_test`) to access unexported helper functions. Four fixture files in `lib/monitor/testdata/`. Three tasks: error-path + V2 end-to-end tests, V3 + IPv6 end-to-end tests, helper-function table tests.

**Tech Stack:** Go standard library (`testing`, `path/filepath`, `time`). No new dependencies.

## Global Constraints

- Test file must be `package monitor` (not `package monitor_test`) — required to access unexported functions `parseConnectedSince` and `splitHostPort`
- All fixture files live under `lib/monitor/testdata/` — Go sets the working directory to the package source dir when running tests, so `"testdata/filename"` resolves correctly
- Run tests with: `export PATH=$PATH:/usr/local/go/bin && go test ./lib/monitor/... -v` from the repo root `/home/karsten/OneDrive/Dokumente/Repos/openvpn-ui`
- No new exported symbols — existing API unchanged
- No mocking — all tests use real file I/O or real function calls

---

### Task 1: Fixture Files + Error-Path + V2 End-to-End Tests

**Files:**
- Create: `lib/monitor/testdata/empty.log`
- Create: `lib/monitor/testdata/v2-basic.log`
- Create: `lib/monitor/statuslog_test.go`

- [ ] **Step 1: Write the test file (RED)**

Create `lib/monitor/statuslog_test.go` with this exact content:

```go
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
```

- [ ] **Step 2: Run tests — verify RED**

```bash
export PATH=$PATH:/usr/local/go/bin
go test ./lib/monitor/... -v -run "TestParseStatusLog"
```

Expected: two failures (fixture files missing), `FileNotFound` passes:
```
--- PASS: TestParseStatusLog_FileNotFound
--- FAIL: TestParseStatusLog_EmptyFile
    statuslog_test.go:XX: expected no error for empty file, got: open testdata/empty.log: no such file or directory
--- FAIL: TestParseStatusLog_V2
    statuslog_test.go:XX: ParseStatusLog: open testdata/v2-basic.log: no such file or directory
FAIL
```

- [ ] **Step 3: Create the fixture files**

Create `lib/monitor/testdata/empty.log` as a completely empty file (0 bytes).

Create `lib/monitor/testdata/v2-basic.log` with this exact content (trailing newline after END):

```
OpenVPN CLIENT LIST
Updated,Thu Jun 20 12:00:00 2026
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since,Connected Since (time_t)
alice,192.168.1.10:51234,102400,204800,Thu Jun 20 10:00:00 2026,1750416000
bob,10.0.0.5:49123,512,1024,Thu Jun 20 11:00:00 2026,1750419600
ROUTING TABLE
Virtual Address,Common Name,Real Address,Last Ref
10.8.0.2,alice,192.168.1.10:51234,Thu Jun 20 12:00:00 2026
10.8.0.3,bob,10.0.0.5:49123,Thu Jun 20 12:00:00 2026
GLOBAL STATS
Max bcast/mcast queue length,0
END
```

- [ ] **Step 4: Run tests — verify GREEN**

```bash
go test ./lib/monitor/... -v -run "TestParseStatusLog"
```

Expected:
```
--- PASS: TestParseStatusLog_FileNotFound
--- PASS: TestParseStatusLog_EmptyFile
--- PASS: TestParseStatusLog_V2
ok  	github.com/OZON08/openvpn-ui/lib/monitor
```

- [ ] **Step 5: Commit**

```bash
git add lib/monitor/testdata/empty.log lib/monitor/testdata/v2-basic.log lib/monitor/statuslog_test.go
git commit -m "test: add statuslog parser tests — error paths and v2 format"
```

---

### Task 2: V3 and IPv6 End-to-End Tests

**Files:**
- Create: `lib/monitor/testdata/v3-basic.log`
- Create: `lib/monitor/testdata/v2-ipv6.log`
- Modify: `lib/monitor/statuslog_test.go` — append two new test functions

**Context from Task 1:** `statuslog_test.go` exists with `package monitor` declaration, `testdata()` helper, and three tests already passing.

- [ ] **Step 1: Write the new tests (RED)**

Append these two functions to `lib/monitor/statuslog_test.go`:

```go
// ---------------------------------------------------------------------------
// ParseStatusLog — v3 format
// ---------------------------------------------------------------------------

func TestParseStatusLog_V3(t *testing.T) {
	clients, err := ParseStatusLog(testdata("v3-basic.log"))
	if err != nil {
		t.Fatalf("ParseStatusLog: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d: %+v", len(clients), clients)
	}
	c := clients[0]
	if c.CommonName != "carol" {
		t.Errorf("CommonName: want %q, got %q", "carol", c.CommonName)
	}
	if c.RealIP != "172.16.0.1" {
		t.Errorf("RealIP: want %q, got %q", "172.16.0.1", c.RealIP)
	}
	if c.RealPort != "60000" {
		t.Errorf("RealPort: want %q, got %q", "60000", c.RealPort)
	}
	if c.BytesReceived != 307200 {
		t.Errorf("BytesReceived: want 307200, got %d", c.BytesReceived)
	}
	if c.BytesSent != 614400 {
		t.Errorf("BytesSent: want 614400, got %d", c.BytesSent)
	}
	// VirtualIP comes from the ROUTING_TABLE line (CLIENT_LIST field[3] is empty).
	if c.VirtualIP != "10.8.0.4" {
		t.Errorf("VirtualIP: want %q, got %q", "10.8.0.4", c.VirtualIP)
	}
	want := time.Unix(1750412400, 0).UTC()
	if !c.ConnectedAt.Equal(want) {
		t.Errorf("ConnectedAt: want %v, got %v", want, c.ConnectedAt)
	}
}

// ---------------------------------------------------------------------------
// ParseStatusLog — IPv6 real address
// ---------------------------------------------------------------------------

func TestParseStatusLog_IPv6Real(t *testing.T) {
	clients, err := ParseStatusLog(testdata("v2-ipv6.log"))
	if err != nil {
		t.Fatalf("ParseStatusLog: %v", err)
	}
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d: %+v", len(clients), clients)
	}
	c := clients[0]
	if c.CommonName != "dave" {
		t.Errorf("CommonName: want %q, got %q", "dave", c.CommonName)
	}
	if c.RealIP != "::1" {
		t.Errorf("RealIP: want %q, got %q", "::1", c.RealIP)
	}
	if c.RealPort != "54321" {
		t.Errorf("RealPort: want %q, got %q", "54321", c.RealPort)
	}
	if c.VirtualIP != "10.8.0.5" {
		t.Errorf("VirtualIP: want %q, got %q", "10.8.0.5", c.VirtualIP)
	}
}
```

- [ ] **Step 2: Run tests — verify RED**

```bash
go test ./lib/monitor/... -v -run "TestParseStatusLog_V3|TestParseStatusLog_IPv6"
```

Expected: both fail with "no such file or directory":
```
--- FAIL: TestParseStatusLog_V3
--- FAIL: TestParseStatusLog_IPv6Real
FAIL
```

- [ ] **Step 3: Create the fixture files**

Create `lib/monitor/testdata/v3-basic.log` with this exact content:

```
TITLE,OpenVPN 2.6.0 x86_64-pc-linux-gnu
TIME,2026-06-20 12:00:00,1750420000
HEADER,CLIENT_LIST,Common Name,Real Address,Virtual Address,Virtual IPv6 Address,Bytes Received,Bytes Sent,Connected Since,Connected Since (time_t),Username,Client ID,Peer ID,Data Channel Cipher
CLIENT_LIST,carol,172.16.0.1:60000,,,307200,614400,2026-06-20 09:00:00,1750412400,carol,3,3,AES-256-GCM
HEADER,ROUTING_TABLE,Virtual Address,Common Name,Real Address,Last Ref,Last Ref (time_t)
ROUTING_TABLE,10.8.0.4,carol,172.16.0.1:60000,2026-06-20 12:00:00,1750420000
GLOBAL_STATS,Max bcast/mcast queue length,0
END
```

Note: `CLIENT_LIST` fields[3] and [4] are empty (two consecutive commas after `172.16.0.1:60000`) so that the VirtualIP is filled by the `ROUTING_TABLE` line, exercising that code path.

Create `lib/monitor/testdata/v2-ipv6.log` with this exact content:

```
OpenVPN CLIENT LIST
Common Name,Real Address,Bytes Received,Bytes Sent,Connected Since,Connected Since (time_t)
dave,[::1]:54321,1024,2048,Thu Jun 20 10:00:00 2026,1750416000
ROUTING TABLE
Virtual Address,Common Name,Real Address,Last Ref
10.8.0.5,dave,[::1]:54321,Thu Jun 20 12:00:00 2026
GLOBAL STATS
Max bcast/mcast queue length,0
END
```

- [ ] **Step 4: Run tests — verify GREEN**

```bash
go test ./lib/monitor/... -v -run "TestParseStatusLog"
```

Expected — all five ParseStatusLog tests pass:
```
--- PASS: TestParseStatusLog_FileNotFound
--- PASS: TestParseStatusLog_EmptyFile
--- PASS: TestParseStatusLog_V2
--- PASS: TestParseStatusLog_V3
--- PASS: TestParseStatusLog_IPv6Real
ok  	github.com/OZON08/openvpn-ui/lib/monitor
```

- [ ] **Step 5: Commit**

```bash
git add lib/monitor/testdata/v3-basic.log lib/monitor/testdata/v2-ipv6.log lib/monitor/statuslog_test.go
git commit -m "test: add statuslog parser tests — v3 format and IPv6 real address"
```

---

### Task 3: Helper Function Table Tests

**Files:**
- Modify: `lib/monitor/statuslog_test.go` — append three table-driven test functions

**Context from Tasks 1–2:** `statuslog_test.go` exists with five end-to-end tests passing. Because the test file is in `package monitor`, the unexported functions `parseConnectedSince` and `splitHostPort` are directly callable.

Note on TDD: these tests exercise existing functions that already work correctly. There is no RED phase — the tests pass as soon as they are written. Write them, run them, verify they pass.

- [ ] **Step 1: Append the three helper tests**

Append these functions to `lib/monitor/statuslog_test.go`:

```go
// ---------------------------------------------------------------------------
// parseConnectedSince — table test
// ---------------------------------------------------------------------------

func TestParseConnectedSince(t *testing.T) {
	cases := []struct {
		input    string
		wantZero bool
	}{
		// All five layouts the function recognises
		{"Thu Jun 20 10:00:00 2026", false},           // time.ANSIC
		{"Thu Jun  5 10:00:00 2026", false},            // double-space day variant
		{"Thu, 20 Jun 2026 10:00:00 UTC", false},       // time.RFC1123
		{"2026-06-20T10:00:00Z", false},                // time.RFC3339
		// Fallback cases
		{"", true},              // empty → zero time
		{"not a timestamp", true}, // garbage → zero time
	}
	for _, tc := range cases {
		got := parseConnectedSince(tc.input)
		if tc.wantZero && !got.IsZero() {
			t.Errorf("parseConnectedSince(%q): want zero time, got %v", tc.input, got)
		}
		if !tc.wantZero && got.IsZero() {
			t.Errorf("parseConnectedSince(%q): want non-zero time, got zero", tc.input)
		}
	}
}

// ---------------------------------------------------------------------------
// splitHostPort — table test
// ---------------------------------------------------------------------------

func TestSplitHostPort(t *testing.T) {
	cases := []struct {
		input, wantHost, wantPort string
	}{
		{"192.168.1.1:51234", "192.168.1.1", "51234"}, // IPv4 with port
		{"[::1]:54321", "::1", "54321"},                // IPv6 bracketed with port
		{"hostname", "hostname", ""},                   // bare host, no port
		{"", "", ""},                                   // empty string
	}
	for _, tc := range cases {
		host, port := splitHostPort(tc.input)
		if host != tc.wantHost || port != tc.wantPort {
			t.Errorf("splitHostPort(%q): want (%q, %q), got (%q, %q)",
				tc.input, tc.wantHost, tc.wantPort, host, port)
		}
	}
}

// ---------------------------------------------------------------------------
// FormatAddr — table test
// ---------------------------------------------------------------------------

func TestFormatAddr(t *testing.T) {
	cases := []struct {
		host, port, want string
	}{
		{"192.168.1.1", "51234", "192.168.1.1:51234"}, // host + port
		{"192.168.1.1", "", "192.168.1.1"},             // host only
		{"", "", ""},                                   // both empty
	}
	for _, tc := range cases {
		got := FormatAddr(tc.host, tc.port)
		if got != tc.want {
			t.Errorf("FormatAddr(%q, %q): want %q, got %q", tc.host, tc.port, tc.want, got)
		}
	}
}
```

- [ ] **Step 2: Run all monitor tests — verify all pass**

```bash
go test ./lib/monitor/... -v
```

Expected — all eight tests pass:
```
--- PASS: TestParseStatusLog_FileNotFound
--- PASS: TestParseStatusLog_EmptyFile
--- PASS: TestParseStatusLog_V2
--- PASS: TestParseStatusLog_V3
--- PASS: TestParseStatusLog_IPv6Real
--- PASS: TestParseConnectedSince
--- PASS: TestSplitHostPort
--- PASS: TestFormatAddr
ok  	github.com/OZON08/openvpn-ui/lib/monitor
```

- [ ] **Step 3: Run the full project test suite**

```bash
go test ./... 2>&1
```

Expected: all testable packages pass, no regressions:
```
ok  	github.com/OZON08/openvpn-ui/controllers
ok  	github.com/OZON08/openvpn-ui/lib
ok  	github.com/OZON08/openvpn-ui/lib/monitor
```

- [ ] **Step 4: Commit**

```bash
git add lib/monitor/statuslog_test.go
git commit -m "test: add table-driven tests for parseConnectedSince, splitHostPort, FormatAddr"
```
