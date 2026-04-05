# MVP Auth Design: Amazon Cognito

## Decision

Use Amazon Cognito User Pools for authentication. Rationale:

- Native AWS integration — fits the planned ALB + API Gateway + Lambda stack
- Handles signup, login, JWT issuance, and token refresh out of the box
- TOTP MFA is a config flip, not a rebuild
- Free tier: 50K MAUs (Cognito Essentials) — negligible cost at this stage
- `amazon-cognito-identity-js` SDK works in vanilla JS without Amplify overhead

Email is the primary identifier. No username, no phone.

---

## AWS Setup (Manual Steps)

### 1. Create a Cognito User Pool

AWS Console → Cognito → Create User Pool:

| Setting | Value |
|---|---|
| Pool name | `ie-user-pool-test` / `ie-user-pool-prod` |
| Sign-in identifier | Email |
| Password policy | Cognito defaults |
| MFA | Disabled (enable TOTP later — see below) |
| Account recovery | Email only |
| Email confirmation | Send code on signup |
| Required attributes | `email` |
| Email delivery | Cognito built-in (upgrade to SES for prod) |

Save the **User Pool ID** — format: `us-east-1_XXXXXXXX`

### 2. Create an App Client

Inside the User Pool → App Integration → App Clients → Create:

| Setting | Value |
|---|---|
| Client name | `ie-web-client` |
| Client type | Public (no secret — browser can't keep secrets) |
| Auth flows | `ALLOW_USER_SRP_AUTH`, `ALLOW_REFRESH_TOKEN_AUTH` |
| OAuth grant types | Authorization code with PKCE |
| Callback URLs | `http://localhost:5173/` (dev), `https://<prod-domain>/` (prod) |
| Sign-out URLs | Same as above |
| Identity providers | Cognito User Pool |
| OAuth scopes | `email`, `openid`, `profile` |

Save the **App Client ID**.

### 3. (Optional) Hosted UI Domain

Cognito → User Pool → App Integration → Domain:

```
Prefix: ie-auth-test
→ https://ie-auth-test.auth.us-east-1.amazoncognito.com
```

Not required if using the SDK directly (no redirect flow).

### 4. Environment Variables

Create `.env.test` and `.env.prod` — **do not commit these files**.

```
VITE_COGNITO_REGION=us-east-1
VITE_COGNITO_USER_POOL_ID=us-east-1_XXXXXXXX
VITE_COGNITO_CLIENT_ID=XXXXXXXXXXXXXXXXXXXXXXXX
VITE_COGNITO_DOMAIN=ie-auth-test.auth.us-east-1.amazoncognito.com
```

Ensure `.env.*` is in `.gitignore`.

---

## Frontend Implementation

### Package

```
amazon-cognito-identity-js
```

No Amplify. This is the low-level SDK — smaller bundle, no magic.

### New files

```
src/auth/
  cognito.js     — Cognito client init, signUp, confirmSignUp, signIn, signOut, refreshSession
  session.js     — Token read/write (localStorage), getAuthHeaders(), isSessionValid()
  ui.js          — Login/signup modal DOM (vanilla JS, matches existing 1920s theme)
  auth.css       — Auth modal styles
```

### Modified files

```
package.json      — add amazon-cognito-identity-js
index.html        — add #auth-overlay container
src/main.js       — auth gate: check session → show login or init app
src/styles.css    — import auth.css (or link in index.html)
```

### Auth Flow

```
Page load
  └─ session.js: check localStorage for valid tokens
       ├─ Valid (access token not expired) → init app
       ├─ Access token expired, refresh token valid → refresh → init app
       └─ No valid session → show login modal

Login modal
  ├─ [Email + password] → cognito.js: initiateAuth (SRP)
  │    ├─ Success → store tokens → hide modal → init app
  │    ├─ USER_NOT_CONFIRMED → show confirm-code input → confirmSignUp → auto-login
  │    └─ Error → display inline error message
  └─ "Create account" toggle → signup form
       └─ [Email + password] → cognito.js: signUp
            └─ Success → show confirm-code input → confirmSignUp → auto-login

Ongoing
  └─ On each API call: if access token expires within 5 min → refresh silently
```

### Token Storage

Tokens are stored in `localStorage` using Cognito's default key format:
`CognitoIdentityServiceProvider.<clientId>.<email>.<tokenType>`

`session.js` exposes:
- `isSessionValid()` → boolean
- `getAccessToken()` → string | null
- `getAuthHeaders()` → `{ Authorization: 'Bearer <accessToken>' }` for API calls
- `clearSession()` → logout

Access token lifetime: 1 hour (Cognito default)
Refresh token lifetime: 30 days (Cognito default)

---

## Server-Side JWT Validation (Future Go Backend)

When the Go API is built, validate the JWT on every request:

1. Extract `Authorization: Bearer <token>` from the request header
2. Fetch the JWKS from:
   `https://cognito-idp.<region>.amazonaws.com/<userPoolId>/.well-known/jwks.json`
3. Verify signature, expiry (`exp`), audience (`aud` = client ID), and issuer (`iss`)
4. Cache the JWKS (it rarely changes); re-fetch on unknown `kid`

Use a Go JWT library such as `github.com/lestrrat-go/jwx`.

---

## Enabling TOTP 2FA (Future)

When ready, enable in Cognito — no frontend rebuild required, just one new challenge case:

1. AWS Console → User Pool → Sign-in experience → MFA → Enable TOTP
2. In `cognito.js`, handle the `SOFTWARE_TOKEN_MFA` challenge in the `initiateAuth` response:
   - Prompt the user for their TOTP code
   - Call `respondToAuthChallenge` with `SOFTWARE_TOKEN_MFA_CODE`
3. For first-time setup: call `associateSoftwareToken` → show QR code → `verifySoftwareToken`

No structural changes to the auth flow.

---

## Verification Checklist

- [ ] `npm run dev` → auth modal appears before the app loads
- [ ] Sign up with a real email → confirmation email arrives → enter code → app loads
- [ ] Refresh page → session persists (tokens in localStorage)
- [ ] Clear localStorage → modal appears again
- [ ] `session.getAuthHeaders()` returns `{ Authorization: 'Bearer eyJ...' }`
- [ ] `.env.*` files are not tracked by git
- [ ] Expired access token is refreshed silently using refresh token
