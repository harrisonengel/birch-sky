import './auth.css';
import { signUp, confirmSignUp, signIn } from './cognito.js';

// Returns a Promise that resolves once the user is authenticated.
export function showAuthModal() {
  return new Promise(resolve => {
    const overlay = document.getElementById('auth-overlay');
    overlay.classList.remove('hidden');

    let state = 'login'; // 'login' | 'signup' | 'confirm'
    let pendingEmail = '';

    function render() {
      overlay.innerHTML = `
        <div class="auth-card">
          <h2>${state === 'login' ? 'Sign In' : state === 'signup' ? 'Create Account' : 'Confirm Email'}</h2>
          ${state === 'confirm' ? `
            <div class="auth-field">
              <label>Confirmation Code</label>
              <input id="auth-code" type="text" placeholder="Enter code from email" autocomplete="one-time-code" />
            </div>
          ` : `
            <div class="auth-field">
              <label>Email</label>
              <input id="auth-email" type="email" placeholder="you@example.com" autocomplete="email" />
            </div>
            <div class="auth-field">
              <label>Password</label>
              <input id="auth-password" type="password" placeholder="••••••••" autocomplete="${state === 'login' ? 'current-password' : 'new-password'}" />
            </div>
          `}
          <div class="auth-error" id="auth-error"></div>
          <button class="auth-submit" id="auth-submit">
            ${state === 'login' ? 'Sign In' : state === 'signup' ? 'Create Account' : 'Confirm'}
          </button>
          ${state !== 'confirm' ? `
            <div class="auth-toggle">
              ${state === 'login'
                ? `No account? <button id="auth-switch">Create one</button>`
                : `Have an account? <button id="auth-switch">Sign in</button>`}
            </div>
          ` : ''}
        </div>
      `;

      document.getElementById('auth-submit').addEventListener('click', handleSubmit);
      document.getElementById('auth-switch')?.addEventListener('click', () => {
        state = state === 'login' ? 'signup' : 'login';
        render();
      });

      if (state !== 'confirm') {
        document.getElementById('auth-email').addEventListener('keydown', onEnter);
        document.getElementById('auth-password').addEventListener('keydown', onEnter);
      } else {
        document.getElementById('auth-code').addEventListener('keydown', onEnter);
      }
    }

    function onEnter(e) {
      if (e.key === 'Enter') handleSubmit();
    }

    function setError(msg) {
      document.getElementById('auth-error').textContent = msg;
    }

    function setLoading(on) {
      document.getElementById('auth-submit').disabled = on;
    }

    async function handleSubmit() {
      setError('');
      setLoading(true);

      try {
        if (state === 'login') {
          const email = document.getElementById('auth-email').value.trim();
          const password = document.getElementById('auth-password').value;
          try {
            await signIn(email, password);
          } catch (err) {
            if (err.code === 'UserNotConfirmedException') {
              pendingEmail = email;
              state = 'confirm';
              render();
              return;
            }
            throw err;
          }
          overlay.classList.add('hidden');
          resolve();

        } else if (state === 'signup') {
          const email = document.getElementById('auth-email').value.trim();
          const password = document.getElementById('auth-password').value;
          await signUp(email, password);
          pendingEmail = email;
          state = 'confirm';
          render();

        } else if (state === 'confirm') {
          const code = document.getElementById('auth-code').value.trim();
          await confirmSignUp(pendingEmail, code);
          // Auto sign-in after confirmation — password not available, prompt login
          state = 'login';
          render();
          setError('Email confirmed. Please sign in.');
        }
      } catch (err) {
        setError(err.message || 'Something went wrong.');
        setLoading(false);
      }
    }

    render();
  });
}
