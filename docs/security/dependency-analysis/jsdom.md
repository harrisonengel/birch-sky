# jsdom — Dependency Security Analysis

- **Date assessed:** 2026-04-07
- **Version installed:** ^29.0.1
- **Risk level:** LOW
- **Purpose:** DOM simulation environment for Vitest

## Findings

- Maintained by Domenic Denicola (Google) and the jsdom team.
- ~28M+ weekly downloads on npm; effectively the standard DOM emulator for JS test runners.
- Mature project, active since 2010.
- No CVEs identified during review specific to the current major release line.
- License: MIT.

## Supply chain notes

- Long history of stable maintainership; no suspicious recent activity.
- Used transitively by virtually every JS testing framework that needs a DOM.

## Caveats

- jsdom is intentionally not a real browser — anything that depends on a real layout/rendering engine (Canvas, WebGL, real network) will not behave identically. This is a correctness consideration, not a security one. We mitigate by running browser-dependent flows under Playwright instead.

## Approval

Auto-approved (low risk) per the global install rule.
