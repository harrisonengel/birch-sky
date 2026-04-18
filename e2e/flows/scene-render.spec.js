import { test, expect } from '@playwright/test';

/**
 * Pixel-region assertions for the canvas scene.
 *
 * The scene renders procedurally generated sprites onto #scene. These tests
 * sample pixel data from regions where sprites must appear and assert that
 * the regions are not pure background. This is a deliberately narrow check —
 * its job is to catch "the building disappeared" / "the agent disappeared"
 * regressions, not to validate exact appearance.
 *
 * Layout (from src/scene.js):
 *   groundY    = height - 40
 *   buildingX  = width - 220   (building sprite is 160×160)
 *   agentStart = x=80, sprite is 64×64
 */

/**
 * Reads pixels from a rectangle of #scene and returns the fraction of pixels
 * whose RGB differs from the canvas background (black, #000).
 */
async function nonBackgroundFraction(page, rect) {
  return await page.evaluate(({ x, y, w, h }) => {
    const canvas = document.getElementById('scene');
    const ctx = canvas.getContext('2d');
    const data = ctx.getImageData(x, y, w, h).data;
    let nonBg = 0;
    const total = data.length / 4;
    for (let i = 0; i < data.length; i += 4) {
      // Background is solid black. Anything with any channel > 0 counts.
      if (data[i] > 0 || data[i + 1] > 0 || data[i + 2] > 0) nonBg++;
    }
    return nonBg / total;
  }, rect);
}

async function getCanvasSize(page) {
  return await page.evaluate(() => {
    const c = document.getElementById('scene');
    return { width: c.width, height: c.height };
  });
}

test.describe('scene canvas rendering', () => {
  test.beforeEach(async ({ page }) => {
    // The scene runs its animation regardless of backend state, but the
    // "building stays drawn after a query" test below triggers runFlow,
    // which hits /api/prepper/start and the harness /agent/enter proxy.
    // Stub both so the test doesn't depend on a live stack.
    await page.route('**/api/prepper/**', async (route) => {
      await route.fulfill({ status: 503, body: '' });
    });
    await page.route(/\/agent\/enter$/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          buy_listings: [
            {
              id: 'lst-scene',
              price: 199,
              listing_description: 'Pricing Index — stub',
              seller: 'StubCo',
            },
          ],
        }),
      });
    });

    await page.goto('/?e2e=1');
    // Let the rAF loop run a few frames so the first render is committed.
    await page.waitForFunction(() => {
      const c = document.getElementById('scene');
      return c && c.width > 0 && c.height > 0;
    });
    await page.waitForTimeout(200);
  });

  test('the IE building is drawn on the right side of the scene', async ({ page }) => {
    const { width, height } = await getCanvasSize(page);

    // The building footprint is roughly:
    //   x: [width-220, width-60]
    //   y: [height-200, height-40]
    // Sample a box well inside that footprint.
    const rect = {
      x: Math.max(0, width - 200),
      y: Math.max(0, height - 180),
      w: 120,
      h: 130,
    };

    // Sanity: the sample window must actually fit inside the canvas. If the
    // canvas is somehow smaller than expected, we want a clear failure rather
    // than a confusing pixel-count one.
    expect(rect.x + rect.w).toBeLessThanOrEqual(width);
    expect(rect.y + rect.h).toBeLessThanOrEqual(height);

    const fraction = await nonBackgroundFraction(page, rect);

    // The building has columns, pediment, body, steps, and "IE" text inside
    // this region. Empirically this is well above 10% non-black. We use 5% as
    // a conservative floor that still catches "building completely missing".
    expect(fraction).toBeGreaterThan(0.05);
  });

  test('the agent sprite is drawn on the left side of the scene', async ({ page }) => {
    const { width, height } = await getCanvasSize(page);

    // Agent starts at x=80, sprite is 64×64, sits on groundY = height-40.
    const rect = {
      x: 70,
      y: Math.max(0, height - 110),
      w: 80,
      h: 70,
    };

    expect(rect.x + rect.w).toBeLessThanOrEqual(width);
    expect(rect.y + rect.h).toBeLessThanOrEqual(height);

    const fraction = await nonBackgroundFraction(page, rect);
    expect(fraction).toBeGreaterThan(0.05);
  });

  test('the building stays drawn after a query is sent', async ({ page }) => {
    // Regression guard: the original "building went missing" bug. Run a full
    // happy-path query and re-check the building region after the flow ends.
    await page.locator('#chat-input').fill('pricing data');
    await page.locator('#chat-send').click();

    await expect(
      page.getByText(/I found several relevant sources/i)
    ).toBeVisible({ timeout: 15_000 });

    // Allow the agent to finish running back and the scene to settle.
    await page.waitForTimeout(300);

    const { width, height } = await getCanvasSize(page);
    const rect = {
      x: Math.max(0, width - 200),
      y: Math.max(0, height - 180),
      w: 120,
      h: 130,
    };
    const fraction = await nonBackgroundFraction(page, rect);
    expect(fraction).toBeGreaterThan(0.05);
  });
});
