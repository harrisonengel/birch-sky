import './styles.css';
import { initScene } from './scene.js';
import { initChat } from './chat.js';
import { initDemoFlow } from './demo-flow.js';

const scene = initScene(document.getElementById('scene'));
const chat = initChat(document.getElementById('chat-panel'));
initDemoFlow(scene, chat);
