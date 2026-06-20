# StatusLog Parser Tests Design

## Goal

Add test coverage for `lib/monitor/statuslog.go` — the OpenVPN status file parser. No external dependencies; all tests run via `go test ./lib/monitor/...`.

## Scope

This spec covers only `lib/monitor/statuslog.go`. The ORM-coupled scraper (`scraper.go`) and cert helper scripts are separate sub-projects.

## Approach

Test file in `package monitor` (not `package monitor_test`) to access unexported helper functions directly. Combines end-to-end fixture-based tests for `ParseStatusLog` with table-driven unit tests for the helper functions.

## Fixtures

Four files in `lib/monitor/testdata/`:

### `v2-basic.log`
v2-format status log with two clients (`alice`, `bob`). Includes:
- Routing table with virtual IPs for both clients
- `time_t` timestamps in the 6th column (locale-free path)
- IPv4 real addresses

### `v3-basic.log`
v3-format status log with one client (`carol`). Includes:
- `CLIENT_LIST,` prefix lines
- `ROUTING_TABLE,` prefix lines with virtual IP
- `TITLE,` and `TIME,` preamble lines (must be ignored)
- `time_t` timestamp in field[8]

### `v2-ipv6.log`
v2-format with one client (`dave`) whose real address is `[::1]:54321`. Tests that `splitHostPort` correctly handles bracketed IPv6 notation.

### `empty.log`
Empty file. Parser must return no error and an empty (non-nil) slice.

## Test File

**`lib/monitor/statuslog_test.go`** — `package monitor`

### End-to-End Tests

| Test | Fixture | Assertions |
|------|---------|------------|
| `TestParseStatusLog_V2` | `v2-basic.log` | 2 clients returned; `alice`: CN, BytesReceived, BytesSent, RealIP, RealPort, VirtualIP, ConnectedAt == `time.Unix(ts, 0).UTC()`; `bob`: same fields |
| `TestParseStatusLog_V3` | `v3-basic.log` | 1 client (`carol`); VirtualIP populated from ROUTING_TABLE; ConnectedAt from time_t field |
| `TestParseStatusLog_IPv6Real` | `v2-ipv6.log` | RealIP == `"::1"`, RealPort == `"54321"` |
| `TestParseStatusLog_EmptyFile` | `empty.log` | err == nil, len(clients) == 0 |
| `TestParseStatusLog_FileNotFound` | `"/nonexistent/path"` | err != nil |

### Helper Unit Tests (Table-Driven)

**`TestParseConnectedSince`** — tests all timestamp layouts the function handles:

| Input | Expected |
|-------|----------|
| `"Thu Jun 20 10:00:00 2026"` (ANSIC) | parsed, non-zero |
| `"Thu Jun  5 10:00:00 2026"` (double-space variant) | parsed, non-zero |
| `"Thu, 20 Jun 2026 10:00:00 UTC"` (RFC1123) | parsed, non-zero |
| `"2026-06-20T10:00:00Z"` (RFC3339) | parsed, non-zero |
| `""` | zero time |
| `"not a timestamp"` | zero time |

**`TestSplitHostPort`** — table test:

| Input | host | port |
|-------|------|------|
| `"192.168.1.1:51234"` | `"192.168.1.1"` | `"51234"` |
| `"[::1]:54321"` | `"::1"` | `"54321"` |
| `"hostname"` | `"hostname"` | `""` |
| `""` | `""` | `""` |

**`TestFormatAddr`** — table test:

| host | port | expected |
|------|------|----------|
| `"192.168.1.1"` | `"51234"` | `"192.168.1.1:51234"` |
| `"192.168.1.1"` | `""` | `"192.168.1.1"` |
| `""` | `""` | `""` |

## Out of Scope

- `scraper.go`, `retention.go`, `influx.go` — ORM/Beego-coupled, separate sub-project
- EasyRSA cert helper script tests — separate sub-project
- No new exported symbols — existing API unchanged
