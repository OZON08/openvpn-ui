# Cert Helper Tests Design

## Goal

Close the remaining gaps in `lib/certificates_test.go`: `ReadCerts` happy path and three missing `parseDetails` field cases. All tests are pure Go, no shell, no easyrsa required.

## Scope

Two targets in `lib/certificates.go`:

### `ReadCerts(path string) ([]*Cert, error)`

Parses `pki/index.txt` — a tab-separated file with exactly 6 fields per line:

```
status  expiry          revocation      serial    filename  subject_dn
V       301215000000Z                   A1B2C3D4  unknown   /CN=alice/name=alice/LocalIP=10.8.0.2/2FAName=none
R       291215000000Z   240601000000Z   A1B2C3D5  unknown   /CN=bob/name=bob
```

Currently only error paths are tested. The happy path — parsing valid and revoked entries, populating `ExpirationT`, `RevocationT`, and calling `parseDetails` — has no coverage.

### `parseDetails(d string) *Details`

Already tested for CN, Country, City, Email. Not tested:
- `LocalIP` field
- `TFAName` (stored as `2FAName` in the DN)
- Name fallback to CN (when no `/name=` field is present, `Details.Name` must equal `Details.CN`)

## What Changes

**New file:** `lib/testdata/index-valid.txt`  
Two-line fixture: one valid cert (with LocalIP and TFAName), one revoked cert (without `/name=`, so Name falls back to CN).

**Modified file:** `lib/certificates_test.go`  
Three new test functions:

| Function | What it asserts |
|---|---|
| `TestReadCerts_ValidFile` | `len==2`, active cert: EntryType, ExpirationT non-zero, CN, LocalIP, TFAName; revoked cert: RevocationT non-zero |
| `TestParseDetails_LocalIPAndTFAName` | `/LocalIP=10.8.0.5/2FAName=mytoken` → `LocalIP=="10.8.0.5"`, `TFAName=="mytoken"` |
| `TestParseDetails_NameFallbackToCN` | `/CN=charlie` (no `/name=`) → `Name=="charlie"` |

## Out of Scope

- Shell scripts (`genclient.sh`, `revoke.sh`, `renew.sh`, `rmcert.sh`)
- `CreateCertificate`, `RevokeCertificate`, `RenewCertificate`, `BurnCertificate` (require shell + easyrsa)
- `buildOpenVPNEnv`, `buildCertEnv` (depend on `state.GlobalCfg` which requires beego init)
