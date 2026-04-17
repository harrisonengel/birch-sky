import './styles.css';
import { initScene } from './scene.js';
import { initChat } from './chat.js';
import { initDemoFlow } from './demo-flow.js';

async function init() {
  // Two bypass paths around the Cognito gate:
  //   - ?e2e=1 URL param: Playwright runs without real credentials.
  //   - VITE_AUTH_MODE=local: local docker-compose demo runs without
  //     a Cognito user pool configured at all.
  //
  // Both paths dynamically import auth modules only when needed, so
  // environments without VITE_COGNITO_* env vars don't crash on
  // synchronous CognitoUserPool construction in auth/cognito.js.
  const e2eBypass = new URLSearchParams(window.location.search).get('e2e') === '1';
  const localBypass = import.meta.env.VITE_AUTH_MODE === 'local';

  if (localBypass) {
    const { applyLocalBypass } = await import('./auth/local-bypass.js');
    applyLocalBypass();
  } else if (!e2eBypass) {
    const { ensureSession } = await import('./auth/session.js');
    const { showAuthModal } = await import('./auth/ui.js');
    const session = await ensureSession();
    if (!session) {
      await showAuthModal();
    }
  }

  const scene = initScene(document.getElementById('scene'));
  const chat = initChat(document.getElementById('chat-panel'));
  initDemoFlow(scene, chat);
}

init();
