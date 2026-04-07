# Go Runtime — Security Analysis

| Field | Value |
|-------|-------|
| Package | `go` (via `brew install go`) |
| Version | 1.26.1 |
| Date | 2026-04-05 |
| Risk Level | **Low** |

## CVEs / Advisories

All known CVEs in Go 1.25.x/1.26.x series are patched in 1.26.1:
- CVE-2026-33809: `golang.org/x/image` TIFF decoding DoS (extended stdlib, not core)
- CVE-2025-61728: `archive/zip` excessive CPU consumption
- CVE-2025-61726: `net/url` memory exhaustion in query parsing
- CVE-2025-68121: `crypto/tls` unexpected session resumption
- CVE-2025-61730: `crypto/tls` handshake encryption level issue
- CVE-2025-61732: `cmd/cgo` code smuggling via doc comments

## Supply Chain Notes

- Distributed as a signed Homebrew core formula (pre-compiled bottle from Google)
- No incidents affecting the Homebrew formula or Go toolchain
- BoltDB typosquat attack (Feb 2025) affected a third-party Go module, not the toolchain

## Conclusion

Google-maintained, widely-audited runtime. Safe to install.
