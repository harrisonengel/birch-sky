import {
  createAgentIdleFrames,
  createAgentRunFrames,
  createBuildingFrames,
} from './sprites.js';

export function initScene(canvas) {
  const ctx = canvas.getContext('2d');

  // Sprite frames
  const agentIdle = createAgentIdleFrames();
  const agentRun = createAgentRunFrames();
  const building = createBuildingFrames();

  // Scene layout constants
  const GROUND_MARGIN = 40;
  const AGENT_START_X = 80;
  const AGENT_SPEED = 250; // pixels per second

  // State
  let width = 0;
  let height = 0;
  let groundY = 0;
  let buildingX = 0;

  // Agent state
  let agentX = AGENT_START_X;
  let agentState = 'idle'; // idle | running-to | running-back
  let agentFrame = 0;
  let agentAnimTimer = 0;
  let runResolve = null;

  // Building state
  let buildingFrame = 0;
  let buildingAnimTimer = 0;
  let buildingWorking = false;

  function resize() {
    const rect = canvas.parentElement.getBoundingClientRect();
    const canvasRect = canvas.getBoundingClientRect();
    width = canvasRect.width || rect.width - 400;
    height = canvasRect.height || rect.height;
    canvas.width = width;
    canvas.height = height;
    groundY = height - GROUND_MARGIN;
    buildingX = width - 220;
  }

  function getAgentIdleInterval() { return 500; }
  function getAgentRunInterval() { return 120; }
  function getBuildingInterval() { return buildingWorking ? 150 : 600; }

  function update(dt) {
    // Agent animation
    agentAnimTimer += dt;

    if (agentState === 'idle') {
      if (agentAnimTimer >= getAgentIdleInterval()) {
        agentAnimTimer = 0;
        agentFrame = (agentFrame + 1) % 2;
      }
    } else {
      // Running
      if (agentAnimTimer >= getAgentRunInterval()) {
        agentAnimTimer = 0;
        agentFrame = (agentFrame + 1) % 4;
      }

      const dir = agentState === 'running-to' ? 1 : -1;
      agentX += dir * AGENT_SPEED * (dt / 1000);

      if (agentState === 'running-to' && agentX >= buildingX - 40) {
        agentX = buildingX - 40;
        agentState = 'idle';
        agentFrame = 0;
        agentAnimTimer = 0;
        if (runResolve) { runResolve(); runResolve = null; }
      } else if (agentState === 'running-back' && agentX <= AGENT_START_X) {
        agentX = AGENT_START_X;
        agentState = 'idle';
        agentFrame = 0;
        agentAnimTimer = 0;
        if (runResolve) { runResolve(); runResolve = null; }
      }
    }

    // Building animation
    buildingAnimTimer += dt;
    if (buildingAnimTimer >= getBuildingInterval()) {
      buildingAnimTimer = 0;
      buildingFrame = (buildingFrame + 1) % 2;
    }
  }

  function render() {
    ctx.clearRect(0, 0, width, height);

    // Background
    ctx.fillStyle = '#000';
    ctx.fillRect(0, 0, width, height);

    // Ground line
    ctx.strokeStyle = '#333';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(0, groundY);
    ctx.lineTo(width, groundY);
    ctx.stroke();

    // Building
    const bFrames = building;
    const bCanvas = bFrames[buildingFrame];
    const bY = groundY - bCanvas.height;
    ctx.drawImage(bCanvas, buildingX, bY);

    // Title above building
    ctx.fillStyle = '#555';
    ctx.font = '11px Courier New';
    ctx.textAlign = 'center';
    ctx.fillText('THE INFORMATION EXCHANGE', buildingX + bCanvas.width / 2, bY - 10);

    // Agent
    const isRunning = agentState === 'running-to' || agentState === 'running-back';
    const aFrames = isRunning ? agentRun : agentIdle;
    const aCanvas = aFrames[agentFrame];
    const aY = groundY - aCanvas.height;

    ctx.save();
    if (agentState === 'running-back') {
      // Flip horizontally
      ctx.translate(agentX + aCanvas.width, aY);
      ctx.scale(-1, 1);
      ctx.drawImage(aCanvas, 0, 0);
    } else {
      ctx.drawImage(aCanvas, agentX, aY);
    }
    ctx.restore();
  }

  // Game loop
  let lastTime = 0;
  function loop(time) {
    const dt = lastTime ? time - lastTime : 16;
    lastTime = time;
    update(dt);
    render();
    requestAnimationFrame(loop);
  }

  // Public API
  function agentRunTo() {
    return new Promise((resolve) => {
      agentState = 'running-to';
      agentFrame = 0;
      agentAnimTimer = 0;
      runResolve = resolve;
    });
  }

  function agentRunBack() {
    return new Promise((resolve) => {
      agentState = 'running-back';
      agentFrame = 0;
      agentAnimTimer = 0;
      runResolve = resolve;
    });
  }

  function setBuildingWorking(working) {
    buildingWorking = working;
    buildingAnimTimer = 0;
  }

  function buildingWork(durationMs) {
    return new Promise((resolve) => {
      setBuildingWorking(true);
      setTimeout(() => {
        setBuildingWorking(false);
        resolve();
      }, durationMs);
    });
  }

  // Init
  resize();
  window.addEventListener('resize', resize);
  requestAnimationFrame(loop);

  return { agentRunTo, agentRunBack, buildingWork, setBuildingWorking };
}
