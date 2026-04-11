# Dependency Analysis: spf13/cobra (and transitive deps)

**Date assessed:** 2026-04-10
**Reason for install:** Standardized Go CLI framework per project rule. Used by the new `cmd/agent` CLI in this PR.

## Packages

| Package | Version | Type | Risk |
|---|---|---|---|
| github.com/spf13/cobra | v1.10.2 | direct | **low** |
| github.com/spf13/pflag | v1.0.9 | transitive (cobra) | **low** |
| github.com/inconshreveable/mousetrap | v1.1.0 | transitive (cobra, Windows-only) | **low** |

## Findings

### github.com/spf13/cobra v1.10.2 — low risk

- No published GitHub security advisories.
- Not present in the Go Vulnerability Database.
- v1.10.2 confirmed legitimate on pkg.go.dev (published 2025-12-03).
- Signed release by maintainer jpmcb.
- cobra/viper are part of the **GitHub Secure Open Source Fund**.
- v1.10.2 release notes explicitly call out supply-chain cleanup
  (migration off deprecated `gopkg.in/yaml.v3`).
- Flagship Go CLI framework with millions of downstream users; very actively
  maintained.

### github.com/spf13/pflag v1.0.9 — low risk

- No CVEs or advisories found.
- v1.0.9 confirmed legitimate (2024-09-01), signed by @tomasaschan.
- Same spf13 org and Secure Open Source Fund umbrella as cobra.
- A 2023 maintenance-status concern was resolved by the active 2024 release
  cadence (v1.0.8 / v1.0.9 / v1.0.10).

### github.com/inconshreveable/mousetrap v1.1.0 — low risk

- No CVEs, advisories, or supply chain incidents.
- v1.1.0 confirmed on pkg.go.dev (2022-11-27).
- ~30 lines of code; Windows-only helper detecting launch-from-Explorer.
- Trivially auditable; tiny attack surface.
- Pulled in only as a Windows transitive dep of cobra. Stale-looking activity
  is appropriate for a feature-complete micro-library.

## Caveats

None. All three packages are well-known, actively maintained (or appropriately
finished, in mousetrap's case), and have clean security histories.

## Recommendation

**Safe to install — no user approval required.** All packages rated low risk
under the INSTALL RULE's medium/high gate.

## Sources

- https://github.com/spf13/cobra/security
- https://github.com/spf13/cobra/releases
- https://github.com/spf13/pflag/releases
- https://github.com/inconshreveable/mousetrap
- https://pkg.go.dev/github.com/spf13/cobra?tab=versions
- https://pkg.go.dev/vuln/list
- https://spf13.com/p/cobra-viper-fortify-security-as-part-of-github-secure-open-source-fund/
