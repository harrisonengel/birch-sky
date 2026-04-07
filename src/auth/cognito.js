import {
  CognitoUserPool,
  CognitoUser,
  AuthenticationDetails,
  CognitoUserAttribute,
} from 'amazon-cognito-identity-js';

const pool = new CognitoUserPool({
  UserPoolId: import.meta.env.VITE_COGNITO_USER_POOL_ID,
  ClientId: import.meta.env.VITE_COGNITO_CLIENT_ID,
});

export function getPool() {
  return pool;
}

export function signUp(email, password) {
  return new Promise((resolve, reject) => {
    const attrs = [new CognitoUserAttribute({ Name: 'email', Value: email })];
    pool.signUp(email, password, attrs, null, (err, result) => {
      if (err) reject(err);
      else resolve(result);
    });
  });
}

export function confirmSignUp(email, code) {
  return new Promise((resolve, reject) => {
    const user = new CognitoUser({ Username: email, Pool: pool });
    user.confirmRegistration(code, true, (err, result) => {
      if (err) reject(err);
      else resolve(result);
    });
  });
}

export function signIn(email, password) {
  return new Promise((resolve, reject) => {
    const user = new CognitoUser({ Username: email, Pool: pool });
    const authDetails = new AuthenticationDetails({ Username: email, Password: password });
    user.authenticateUser(authDetails, {
      onSuccess(session) { resolve(session); },
      onFailure(err) { reject(err); },
      newPasswordRequired(userAttributes) {
        // Admin-created users must set a permanent password on first login.
        // Complete the challenge with the same password they entered.
        delete userAttributes.email_verified;
        delete userAttributes.email;
        user.completeNewPasswordChallenge(password, userAttributes, {
          onSuccess(session) { resolve(session); },
          onFailure(err) { reject(err); },
        });
      },
    });
  });
}

export function signOut() {
  const user = pool.getCurrentUser();
  if (user) user.signOut();
}
