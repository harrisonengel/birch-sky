# Dependency Analysis: Python agent harness deps

**Date assessed:** 2026-04-10
**Reason for install:** New `harness/` package (buyer-agent runner).

## Packages

| Package | Version pin | Risk | Notes |
|---|---|---|---|
| anthropic | >=0.87.0 | **low** | Official Anthropic Python SDK |
| pyyaml | >=6.0.2 | **low** | Standard YAML parser |
| requests | >=2.33.0 | **low** | Standard HTTP client |

## Findings

### anthropic >=0.87.0 — low risk

- Official first-party SDK published by Anthropic, PBC.
- No current CVEs at pinned floor. Two historical CVEs
  (CVE-2026-34450 world-readable tmp files, CVE-2026-34452 TOCTOU
  symlink race) both scoped to the optional local-filesystem memory
  tool, fixed in 0.87.0. We do not use that tool.
- Healthy maintainer activity; releases cut frequently.
- Transitive deps (httpx, pydantic, anyio, distro, jiter, sniffio,
  typing-extensions) all mainstream, no active advisories.
- No supply chain incidents in last 6 months.

### pyyaml >=6.0.2 — low risk

- Recent CVEs (CVE-2026-24009, CVE-2025-50460) are **usage bugs**:
  they require calling `yaml.load()` on untrusted input. The harness
  exclusively uses `yaml.safe_load()` (`harness/config.py`).
- Mature, widely used, no supply chain concerns.

### requests >=2.33.0 — low risk

- CVE-2026-25645 affects the rarely used `extract_zipped_paths()`
  helper — not standard usage. Fixed in 2.33.0.
- Transitive urllib3 decompression-bomb DoS (CVE-2026-21441) fixed
  in urllib3 2.6.3; will be resolved by a normal pip install.
- Industry-standard HTTP client, no supply chain concerns.

## Rejected alternatives

`openai-agents[litellm]` was the original design choice but was rejected
after discovering the **LiteLLM supply chain compromise** (March 24, 2026):
versions 1.82.7 and 1.82.8 were trojanized with a multi-stage credential
stealer (TeamPCP threat actor, poisoned Trivy scanner in CI/CD). Clean
versions exist (≤1.82.6, ≥1.83.0) but the recently-rebuilt pipeline is
unproven. Writing a ~30-line tool loop against the Anthropic SDK directly
eliminates both `litellm` and `openai-agents` from the dependency tree.

## Recommendation

**Safe to install** — all packages rated low risk under the INSTALL RULE.
No user approval required at the pinned floors above.
