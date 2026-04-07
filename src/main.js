import './styles.css';
import { initScene } from './scene.js';
import { initChat } from './chat.js';
import { initDemoFlow } from './demo-flow.js';

async function init() {
  // E2E bypass: skip the Cognito gate when ?e2e=1 is set so Playwright can
  // exercise the demo flow without real credentials. Mirrors the #demo-mode
  // testing hook used by demo-flow.js.
  //
  // The auth modules are dynamically imported so that test environments
  // without VITE_COGNITO_* env vars don't crash on the synchronous
  // CognitoUserPool construction in auth/cognito.js.
  const e2eBypass = new URLSearchParams(window.location.search).get('e2e') === '1';

  if (!e2eBypass) {
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
