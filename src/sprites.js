// Generates placeholder sprite frames as offscreen canvases.
// Each function returns an array of canvases (one per frame).

function createCanvas(w, h) {
  const c = document.createElement('canvas');
  c.width = w;
  c.height = h;
  return c;
}

function drawPixel(ctx, x, y, size, color) {
  ctx.fillStyle = color;
  ctx.fillRect(x * size, y * size, size, size);
}

// Draw a filled rectangle in pixel-grid coordinates
function drawRect(ctx, px, py, pw, ph, size, color) {
  ctx.fillStyle = color;
  ctx.fillRect(px * size, py * size, pw * size, ph * size);
}

export function createAgentIdleFrames() {
  const S = 2; // pixel scale
  const frames = [];

  for (let f = 0; f < 2; f++) {
    const c = createCanvas(64, 64);
    const ctx = c.getContext('2d');
    const offsetY = f === 1 ? -1 : 0; // bounce up 1 grid unit

    // Hat (fedora)
    drawRect(ctx, 11, 3 + offsetY, 10, 2, S, '#aaa');
    drawRect(ctx, 12, 1 + offsetY, 8, 2, S, '#999');

    // Head
    drawRect(ctx, 13, 5 + offsetY, 6, 6, S, '#ccc');

    // Body (suit jacket)
    drawRect(ctx, 11, 11 + offsetY, 10, 10, S, '#888');
    // Lapels
    drawRect(ctx, 14, 11 + offsetY, 1, 5, S, '#666');
    drawRect(ctx, 17, 11 + offsetY, 1, 5, S, '#666');

    // Arms
    drawRect(ctx, 9, 12 + offsetY, 2, 8, S, '#777');
    drawRect(ctx, 21, 12 + offsetY, 2, 8, S, '#777');

    // Legs
    drawRect(ctx, 12, 21 + offsetY, 3, 8, S, '#666');
    drawRect(ctx, 17, 21 + offsetY, 3, 8, S, '#666');

    // Shoes
    drawRect(ctx, 11, 29 + offsetY, 4, 2, S, '#555');
    drawRect(ctx, 17, 29 + offsetY, 4, 2, S, '#555');

    frames.push(c);
  }
  return frames;
}

export function createAgentRunFrames() {
  const S = 2;
  const frames = [];
  // 4 frames: stride positions
  const legPositions = [
    // [leftLegX, leftLegExtend, rightLegX, rightLegExtend]
    { ll: -3, rl: 3 },   // left back, right forward
    { ll: -1, rl: 1 },   // passing center
    { ll: 3, rl: -3 },   // left forward, right back
    { ll: 1, rl: -1 },   // passing center
  ];

  for (let f = 0; f < 4; f++) {
    const c = createCanvas(64, 64);
    const ctx = c.getContext('2d');
    const lp = legPositions[f];

    // Lean forward: shift top slightly right
    const lean = 1;

    // Hat
    drawRect(ctx, 11 + lean, 3, 10, 2, S, '#aaa');
    drawRect(ctx, 12 + lean, 1, 8, 2, S, '#999');

    // Head
    drawRect(ctx, 13 + lean, 5, 6, 6, S, '#ccc');

    // Body (leaning)
    drawRect(ctx, 11 + lean, 11, 10, 10, S, '#888');
    drawRect(ctx, 14 + lean, 11, 1, 5, S, '#666');
    drawRect(ctx, 17 + lean, 11, 1, 5, S, '#666');

    // Arms (swinging opposite to legs)
    drawRect(ctx, 9 + lean - lp.rl, 12, 2, 8, S, '#777');
    drawRect(ctx, 21 + lean - lp.ll, 12, 2, 8, S, '#777');

    // Left leg
    drawRect(ctx, 12 + lp.ll, 21, 3, 8, S, '#666');
    drawRect(ctx, 11 + lp.ll, 29, 4, 2, S, '#555');

    // Right leg
    drawRect(ctx, 17 + lp.rl, 21, 3, 8, S, '#666');
    drawRect(ctx, 17 + lp.rl, 29, 4, 2, S, '#555');

    frames.push(c);
  }
  return frames;
}

export function createBuildingFrames() {
  const S = 2;
  const frames = [];

  for (let f = 0; f < 2; f++) {
    const c = createCanvas(160, 160);
    const ctx = c.getContext('2d');
    const offsetY = f === 1 ? -1 : 0;

    // Steps (3 tiers)
    drawRect(ctx, 10, 68 + offsetY, 60, 3, S, '#555');
    drawRect(ctx, 13, 65 + offsetY, 54, 3, S, '#666');
    drawRect(ctx, 16, 62 + offsetY, 48, 3, S, '#777');

    // Main building body
    drawRect(ctx, 18, 22 + offsetY, 44, 40, S, '#666');

    // Columns (6 columns)
    for (let i = 0; i < 6; i++) {
      const cx = 21 + i * 7;
      drawRect(ctx, cx, 24 + offsetY, 2, 36, S, '#999');
    }

    // Pediment (triangle)
    ctx.fillStyle = '#888';
    ctx.beginPath();
    ctx.moveTo(16 * S, (22 + offsetY) * S);
    ctx.lineTo(40 * S, (10 + offsetY) * S);
    ctx.lineTo(64 * S, (22 + offsetY) * S);
    ctx.closePath();
    ctx.fill();

    // Pediment border
    ctx.strokeStyle = '#aaa';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.moveTo(16 * S, (22 + offsetY) * S);
    ctx.lineTo(40 * S, (10 + offsetY) * S);
    ctx.lineTo(64 * S, (22 + offsetY) * S);
    ctx.closePath();
    ctx.stroke();

    // "IE" text on building
    ctx.fillStyle = '#ddd';
    ctx.font = 'bold 20px Courier New';
    ctx.textAlign = 'center';
    ctx.fillText('IE', 40 * S, (45 + offsetY) * S);

    frames.push(c);
  }
  return frames;
}
