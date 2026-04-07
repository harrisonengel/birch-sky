# Dependency Security Analysis: `anthropic`

**Date assessed:** 2026-04-07
**Package:** `anthropic` (PyPI)
**Version range pinned:** `>=0.87.0`
**Risk level:** **LOW**
**Assessed by:** Claude Code (general-purpose sub-agent), per global INSTALL RULE

## Summary

The official first-party Python SDK from Anthropic, PBC. Published from `anthropics/anthropic-sdk-python` on GitHub, auto-generated via Stainless. Long, consistent release cadence with active security response. Approved for use after pinning above the patched-CVE floor.

## CVEs / Advisories

| CVE | Severity | Affected | Fixed in | Notes |
|---|---|---|---|---|
| CVE-2026-34450 | Local-only | 0.86.0 | **0.87.0** | Local filesystem memory tool created files with mode 0o666 (world-readable/writable under permissive umask). Scoped to the optional memory tool. |
| CVE-2026-34452 | Local-only | 0.86.0 | **0.87.0** | Async memory tool path-validation TOCTOU race allows symlink-based sandbox escape. Only affects users of the async local memory tool. |

Both CVEs are local-only, scoped to the optional local memory tool, and patched in 0.87.0. The orchestrator does not use the local memory tool, so impact would be nil even on 0.86.0 — but pinning above ensures safety regardless.

## Supply Chain Notes

- No compromised maintainers
- No typosquatting concerns at this exact name
- No malicious versions on PyPI
- Verified legitimate publisher (Anthropic, PBC)
- (Note: unrelated `litellm` and `telnyx` PyPI compromises surfaced in searches but do not affect `anthropic`.)

## Dependencies

| Package | Notes |
|---|---|
| `httpx` | Standard async HTTP client |
| `pydantic` | Data validation |
| `anyio` | Async abstraction |
| `distro` | OS distribution detection |
| `jiti` / `tokenizers` | Tokenization |
| `sniffio` | Async runtime detection |
| `typing-extensions` | Backports |

All well-known, widely audited Python ecosystem packages.

## Recommendation

**Approved.** Pin `anthropic>=0.87.0` in `requirements.txt`. Re-review if upgrading to a major version bump (e.g. 1.0).

## Sources

- https://advisories.gitlab.com/pkg/pypi/anthropic/CVE-2026-34450/
- https://advisories.gitlab.com/pkg/pypi/anthropic/CVE-2026-34452/
- https://github.com/anthropics/anthropic-sdk-python/releases
- https://getsafety.com/packages/pypi/anthropic/
- https://advisories.gitlab.com/pkg/pypi/anthropic/
