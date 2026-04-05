import './styles.css';
import { ensureSession } from './auth/session.js';
import { showAuthModal } from './auth/ui.js';
import { initScene } from './scene.js';
import { initChat } from './chat.js';
import { initDemoFlow } from './demo-flow.js';

async function init() {
  const session = await ensureSession();
  if (!session) {
    await showAuthModal();
  }

  const scene = initScene(document.getElementById('scene'));
  const chat = initChat(document.getElementById('chat-panel'));
  initDemoFlow(scene, chat);
}

init();
