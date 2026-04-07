# Security Analysis: amazon-cognito-identity-js

**Version assessed:** 6.3.16
**Date assessed:** 2026-04-05
**Risk level:** LOW
**Approved by:** harrisonengel

---

## CVEs and Advisories

No direct CVEs or npm security advisories found in OSV or Snyk.

**Note on CVE-2024-28056 (CVSS 9.8):** Frequently associated in search results with "AWS Amplify + Cognito" but is NOT a vulnerability in this package. It is a misconfiguration bug in the AWS Amplify CLI (pre-12.10.1) affecting IAM trust policies. Not applicable here.

---

## Supply Chain

Published under the `aws-amplify/amplify-js` monorepo. Primary publisher is `aws-amplify-ops` (amazon.com corporate service account). Four personal gmail-linked maintainer accounts retain npm publish rights from the project's early history — none show signs of compromise. Actual publishing routes through the corporate account.

---

## Typosquatting

Checked common variants (`amazon-cognito-identiy-js`, `amazon-cognito-identity`, `cognito-identity-js`, etc.) — none registered on npm.

---

## Dependencies

| Package | Version | Vulnerabilities |
|---|---|---|
| buffer | 4.9.2 | None (pinned, older version) |
| js-cookie | ^2.2.1 | None |
| fast-base64-decode | ^1.0.0 | None |
| isomorphic-unfetch | ^3.0.0 | None |
| @aws-crypto/sha256-js | 1.2.2 | None |

---

## Notes

- The standalone `amazon-archives/amazon-cognito-identity-js` GitHub repo is archived; the package is actively maintained within the `aws-amplify/amplify-js` monorepo and receives security patches.
- AWS's long-term direction is to consolidate on `aws-amplify` as the user-facing Auth SDK. This package remains supported but is increasingly the engine under the hood.
- Run `npm audit` after install to catch any indirect advisory resolutions in your specific lockfile.
