import {
  createAgentIdleFrames,
  createAgentRunFrames,
  createBuildingFrames,
} from './sprites.js';

export function initScene(canvas) {
  const ctx = canvas.getContext('2d');

  const agentIdle = createAgentIdleFrames();
  const agentRun = createAgentRunFrames();
  const building = createBuildingFrames();

  const GROUND_MARGIN = 40;
  const AGENT_SPEED = 250;

  // Cloning-machine layout (canvas-local coordinates)
  const LEFT_PAD_X = 48;
  const RIGHT_PAD_X = 208;
  const PAD_WIDTH = 64;
  const PAD_HEIGHT = 10;
  const WALL_X = 140;
  const WALL_WIDTH = 12;
  const WALL_HEIGHT = 90;

  // Agent and clone centers
  const AGENT_START_X = LEFT_PAD_X + PAD_WIDTH / 2;   // 80
  const CLONE_START_X = RIGHT_PAD_X + PAD_WIDTH / 2;  // 240

  // Shock / materialize / dissolve timings
  const SHOCK_DURATION = 500;
  const MATERIALIZE_DURATION = 400;
  const DISSOLVE_DURATION = 300;

  let width = 0;
  let height = 0;
  let groundY = 0;
  let buildingX = 0;

  // Original agent — permanently on the left pad
  let agentIdleFrame = 0;
  let agentIdleTimer = 0;
  let agentShocking = false;
  let shockTimer = 0;

  // Clone
  let cloneState = 'hidden'; // hidden | materializing | running-to | idle-at-building | running-back | dissolving
  let cloneX = CLONE_START_X;
  let cloneFrame = 0;
  let cloneAnimTimer = 0;
  let cloneFadeTimer = 0;
  let runResolve = null;

  // Machine glow pulse
  let machineFrame = 0;
  let machineAnimTimer = 0;

  // Building
  let buildingFrame = 0;
  let buildingAnimTimer = 0;
  let buildingWorking = false;

  function resize() {
    const rect = canvas.getBoundingClientRect();
    width = Math.max(rect.width, 1);
    height = Math.max(rect.height, 1);
    canvas.width = width;
    canvas.height = height;
    groundY = height - GROUND_MARGIN;
    buildingX = width - 220;
  }

  function update(dt) {
    agentIdleTimer += dt;
    if (agentIdleTimer >= 500) {
      agentIdleTimer = 0;
      agentIdleFrame = (agentIdleFrame + 1) % 2;
    }

    if (agentShocking) {
      shockTimer += dt;
      if (shockTimer >= SHOCK_DURATION) {
        agentShocking = false;
      }
    }

    machineAnimTimer += dt;
    if (machineAnimTimer >= 400) {
      machineAnimTimer = 0;
      machineFrame = (machineFrame + 1) % 2;
    }

    if (cloneState === 'materializing') {
      cloneFadeTimer += dt;
      if (cloneFadeTimer >= MATERIALIZE_DURATION) {
        cloneFadeTimer = 0;
        cloneState = 'running-to';
        cloneFrame = 0;
        cloneAnimTimer = 0;
      }
    } else if (cloneState === 'dissolving') {
      cloneFadeTimer += dt;
      if (cloneFadeTimer >= DISSOLVE_DURATION) {
        cloneFadeTimer = 0;
        cloneState = 'hidden';
        if (runResolve) { runResolve(); runResolve = null; }
      }
    } else if (cloneState === 'running-to') {
      cloneAnimTimer += dt;
      if (cloneAnimTimer >= 120) {
        cloneAnimTimer = 0;
        cloneFrame = (cloneFrame + 1) % 4;
      }
      cloneX += AGENT_SPEED * (dt / 1000);
      if (cloneX >= buildingX - 40) {
        cloneX = buildingX - 40;
        cloneState = 'idle-at-building';
        cloneFrame = 0;
        cloneAnimTimer = 0;
        if (runResolve) { runResolve(); runResolve = null; }
      }
    } else if (cloneState === 'running-back') {
      cloneAnimTimer += dt;
      if (cloneAnimTimer >= 120) {
        cloneAnimTimer = 0;
        cloneFrame = (cloneFrame + 1) % 4;
      }
      cloneX -= AGENT_SPEED * (dt / 1000);
      if (cloneX <= CLONE_START_X) {
        cloneX = CLONE_START_X;
        cloneState = 'dissolving';
        cloneFadeTimer = 0;
        cloneFrame = 0;
        cloneAnimTimer = 0;
      }
    }

    buildingAnimTimer += dt;
    const bInterval = buildingWorking ? 150 : 600;
    if (buildingAnimTimer >= bInterval) {
      buildingAnimTimer = 0;
      buildingFrame = (buildingFrame + 1) % 2;
    }
  }

  function drawMachine() {
    // Cyan glow alternates, brighter while the agent is being cloned
    const bright = agentShocking || cloneState === 'materializing';
    const glow = bright
      ? (machineFrame === 0 ? '#7ff' : '#0ff')
      : (machineFrame === 0 ? '#0cc' : '#088');

    const padTop = groundY - PAD_HEIGHT;

    // Pads
    ctx.fillStyle = '#555';
    ctx.fillRect(LEFT_PAD_X, padTop, PAD_WIDTH, PAD_HEIGHT);
    ctx.fillRect(RIGHT_PAD_X, padTop, PAD_WIDTH, PAD_HEIGHT);
    ctx.fillStyle = '#222';
    ctx.fillRect(LEFT_PAD_X + 4, padTop + 3, PAD_WIDTH - 8, 2);
    ctx.fillRect(RIGHT_PAD_X + 4, padTop + 3, PAD_WIDTH - 8, 2);
    ctx.fillStyle = glow;
    ctx.fillRect(LEFT_PAD_X + 2, padTop - 3, PAD_WIDTH - 4, 3);
    ctx.fillRect(RIGHT_PAD_X + 2, padTop - 3, PAD_WIDTH - 4, 3);

    // Wall separating the two pads
    const wallTop = groundY - WALL_HEIGHT;
    ctx.fillStyle = '#666';
    ctx.fillRect(WALL_X, wallTop, WALL_WIDTH, WALL_HEIGHT);
    ctx.fillStyle = '#333';
    ctx.fillRect(WALL_X + 2, wallTop + 2, WALL_WIDTH - 4, WALL_HEIGHT - 4);
    ctx.fillStyle = glow;
    ctx.fillRect(WALL_X, wallTop - 3, WALL_WIDTH, 3);

    // Tube: two risers and a horizontal arch connecting the pads over the wall
    const tubeY = wallTop - 18;
    const tubeThickness = 8;
    const leftRiserX = LEFT_PAD_X + PAD_WIDTH - 10;
    const rightRiserX = RIGHT_PAD_X + 4;
    ctx.fillStyle = '#666';
    ctx.fillRect(leftRiserX, tubeY, 6, padTop - tubeY);
    ctx.fillRect(rightRiserX, tubeY, 6, padTop - tubeY);
    ctx.fillRect(leftRiserX, tubeY, rightRiserX + 6 - leftRiserX, tubeThickness);
    // Inner dark line + glow stripe
    ctx.fillStyle = '#222';
    ctx.fillRect(leftRiserX + 1, tubeY + 1, rightRiserX + 5 - leftRiserX, tubeThickness - 2);
    ctx.fillStyle = glow;
    ctx.fillRect(leftRiserX + 1, tubeY + 3, rightRiserX + 5 - leftRiserX, 2);

    // A little energy packet travels L→R through the tube while cloning
    if (bright) {
      const travelT = Math.min(1,
        agentShocking
          ? shockTimer / SHOCK_DURATION
          : (SHOCK_DURATION + cloneFadeTimer) / (SHOCK_DURATION + MATERIALIZE_DURATION)
      );
      const packetX = leftRiserX + travelT * (rightRiserX + 6 - leftRiserX - 8);
      ctx.fillStyle = '#fff';
      ctx.fillRect(packetX, tubeY + 1, 8, tubeThickness - 2);
    }
  }

  function drawShockOn(sx, sy, sw, sh, color) {
    ctx.strokeStyle = color;
    ctx.lineWidth = 2;
    const phase = Math.floor(shockTimer / 70) % 2;
    const jitter = phase === 0 ? 0 : 2;
    // Left bolt
    ctx.beginPath();
    ctx.moveTo(sx - 6, sy + 10 + jitter);
    ctx.lineTo(sx + 4, sy + 22 - jitter);
    ctx.lineTo(sx - 4, sy + 34 + jitter);
    ctx.lineTo(sx + 6, sy + 46 - jitter);
    ctx.stroke();
    // Right bolt
    ctx.beginPath();
    ctx.moveTo(sx + sw + 6, sy + 12 - jitter);
    ctx.lineTo(sx + sw - 4, sy + 24 + jitter);
    ctx.lineTo(sx + sw + 4, sy + 36 - jitter);
    ctx.lineTo(sx + sw - 6, sy + 48 + jitter);
    ctx.stroke();
    // Top arc
    ctx.beginPath();
    ctx.moveTo(sx + 6, sy - 4 - jitter);
    ctx.lineTo(sx + sw / 2, sy + 2 + jitter);
    ctx.lineTo(sx + sw - 6, sy - 4 - jitter);
    ctx.stroke();
  }

  function render() {
    ctx.clearRect(0, 0, width, height);
    ctx.fillStyle = '#000';
    ctx.fillRect(0, 0, width, height);

    // Ground line
    ctx.strokeStyle = '#333';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.moveTo(0, groundY);
    ctx.lineTo(width, groundY);
    ctx.stroke();

    drawMachine();

    // Building
    const bCanvas = building[buildingFrame];
    const bY = groundY - bCanvas.height;
    ctx.drawImage(bCanvas, buildingX, bY);
    ctx.fillStyle = '#555';
    ctx.font = '11px Courier New';
    ctx.textAlign = 'center';
    ctx.fillText('THE INFORMATION EXCHANGE', buildingX + bCanvas.width / 2, bY - 10);

    // Original agent on left pad
    const aCanvas = agentIdle[agentIdleFrame];
    const aY = groundY - PAD_HEIGHT - aCanvas.height;
    const aX = AGENT_START_X - aCanvas.width / 2;
    ctx.drawImage(aCanvas, aX, aY);
    if (agentShocking) {
      drawShockOn(aX, aY, aCanvas.width, aCanvas.height, '#ff0');
      // Sympathetic shock on the receiving pad
      drawShockOn(
        RIGHT_PAD_X + PAD_WIDTH / 2 - aCanvas.width / 2,
        aY,
        aCanvas.width,
        aCanvas.height,
        '#0ff'
      );
    }

    // Clone
    if (cloneState !== 'hidden') {
      let opacity = 1;
      if (cloneState === 'materializing') {
        opacity = cloneFadeTimer / MATERIALIZE_DURATION;
      } else if (cloneState === 'dissolving') {
        opacity = 1 - cloneFadeTimer / DISSOLVE_DURATION;
      }

      const isRunning = cloneState === 'running-to' || cloneState === 'running-back';
      const cFrames = isRunning ? agentRun : agentIdle;
      const cIdx = isRunning ? cloneFrame : 0;
      const cCanvas = cFrames[cIdx];
      const cY = groundY - PAD_HEIGHT - cCanvas.height;
      const cX = cloneX - cCanvas.width / 2;

      ctx.save();
      ctx.globalAlpha = Math.max(0, Math.min(1, opacity));
      if (cloneState === 'running-back') {
        ctx.translate(cX + cCanvas.width, cY);
        ctx.scale(-1, 1);
        ctx.drawImage(cCanvas, 0, 0);
      } else {
        ctx.drawImage(cCanvas, cX, cY);
      }
      ctx.restore();

      // Materialization sparkle
      if (cloneState === 'materializing') {
        const t = cloneFadeTimer / MATERIALIZE_DURATION;
        ctx.strokeStyle = `rgba(120, 255, 255, ${1 - t})`;
        ctx.lineWidth = 1;
        for (let i = 0; i < 4; i++) {
          const angle = (i / 4) * Math.PI * 2 + t * Math.PI;
          const r = 20 + t * 20;
          const sx = cloneX + Math.cos(angle) * r;
          const sy = cY + cCanvas.height / 2 + Math.sin(angle) * r * 0.6;
          ctx.beginPath();
          ctx.moveTo(sx - 3, sy);
          ctx.lineTo(sx + 3, sy);
          ctx.moveTo(sx, sy - 3);
          ctx.lineTo(sx, sy + 3);
          ctx.stroke();
        }
      }
    }
  }

  let lastTime = 0;
  function loop(time) {
    const dt = lastTime ? time - lastTime : 16;
    lastTime = time;
    update(dt);
    render();
    requestAnimationFrame(loop);
  }

  // Phase 1 of a query: shock the agent, materialize a clone on the right
  // pad, then run the clone to the market. Resolves when the clone arrives.
  function agentRunTo() {
    return new Promise((resolve) => {
      agentShocking = true;
      shockTimer = 0;
      cloneState = 'materializing';
      cloneX = CLONE_START_X;
      cloneFadeTimer = 0;
      runResolve = resolve;
    });
  }

  // Phase 2: clone runs back to the right pad and dissolves.
  function agentRunBack() {
    return new Promise((resolve) => {
      cloneState = 'running-back';
      cloneFrame = 0;
      cloneAnimTimer = 0;
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

  resize();
  window.addEventListener('resize', resize);
  if (typeof ResizeObserver !== 'undefined') {
    const ro = new ResizeObserver(resize);
    ro.observe(canvas);
  }
  requestAnimationFrame(loop);

  return { agentRunTo, agentRunBack, buildingWork, setBuildingWorking };
}
