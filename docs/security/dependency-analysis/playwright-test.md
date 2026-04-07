# @playwright/test — Dependency Security Analysis

- **Date assessed:** 2026-04-07
- **Version installed:** ^1.59.1
- **Risk level:** LOW
- **Purpose:** End-to-end browser test runner

## Findings

- Maintained by Microsoft with a dedicated team.
- ~7M+ weekly downloads on npm.
- No known CVEs or supply chain incidents identified during review.
- Regular releases with security patches.
- License: Apache 2.0.

## Supply chain notes

- `npx playwright install` downloads browser binaries from `cdn.playwright.dev`. This is standard, expected behavior for browser automation tooling, but worth noting: the install step fetches large signed archives (~165 MiB Chrome) outside of npm's package supply chain.
- Browsers are cached under `~/Library/Caches/ms-playwright/` (macOS).

## Caveats

- Dev-only dependency. Not shipped to production.
- CI must run `npx playwright install --with-deps chromium` before `playwright test` (already wired into `.github/workflows/test.yml`).

## Approval

Auto-approved (low risk) per the global install rule.
