// Dev-mode auth bypass. Active when VITE_AUTH_MODE=local.
//
// Seeds a stable stub buyer identity in sessionStorage so the rest of the
// app (api-client.js, demo-flow.js) can keep calling getBuyerID() without
// caring whether Cognito is wired up. This path must never ship to
// production — the Cognito gate in main.js is bypassed entirely.

export function applyLocalBypass() {
  const existing = sessionStorage.getItem('ie_buyer_id');
  if (!existing) {
    sessionStorage.setItem('ie_buyer_id', 'buyer-local-' + Math.random().toString(36).slice(2, 10));
  }
}
