import { getPool, signOut } from './cognito.js';

function clientId() {
  return getPool().getClientId();
}

function lastAuthUser() {
  return localStorage.getItem(`CognitoIdentityServiceProvider.${clientId()}.LastAuthUser`);
}

function getStoredToken(type) {
  const user = lastAuthUser();
  if (!user) return null;
  return localStorage.getItem(`CognitoIdentityServiceProvider.${clientId()}.${user}.${type}`);
}

function jwtExpiry(token) {
  try {
    return JSON.parse(atob(token.split('.')[1])).exp * 1000;
  } catch {
    return 0;
  }
}

export function getAccessToken() {
  return getStoredToken('accessToken');
}

export function getAuthHeaders() {
  const token = getAccessToken();
  return token ? { Authorization: `Bearer ${token}` } : {};
}

export function clearSession() {
  signOut();
}

// Returns a valid Cognito session (refreshing if access token is expired),
// or null if there is no session or the refresh token is expired.
export function ensureSession() {
  const user = getPool().getCurrentUser();
  if (!user) return Promise.resolve(null);
  return new Promise(resolve => {
    user.getSession((err, session) => {
      if (err || !session || !session.isValid()) resolve(null);
      else resolve(session);
    });
  });
}
