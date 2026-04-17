import { test, expect } from '@playwright/test';

// These tests cover the agent-prepper integration in src/demo-flow.js. The
// real agent-prepper service is not running during e2e, so we mock its HTTP
// surface with page.route. The harness /agent/enter endpoint is also mocked
// so the test stays isolated from harness/market-platform availability.

test.describe('Prepper clarification flow', () => {
  test('multi-turn clarification then results, with briefing folded into the search query', async ({ page }) => {
    let respondCallCount = 0;
    let enterPayload = null;

    await page.route('**/api/prepper/start', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          session_id: 'sess-test-1',
          status: 'asking',
          turn: 1,
          question: 'What region should the data cover?',
        }),
      });
    });

    await page.route('**/api/prepper/respond', async (route) => {
      respondCallCount += 1;
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          session_id: 'sess-test-1',
          status: 'ready',
          turn: 2,
          briefing: {
            goal_summary: 'Track US grocery prices weekly',
            selection_criteria: ['US', 'weekly cadence', 'national chains'],
            analysis_mode: 'evaluate_then_decide',
            background: '',
            constraints: '',
          },
        }),
      });
    });

    // Stub the harness enter so we can assert the briefing was folded in.
    await page.route(/\/agent\/enter$/, async (route) => {
      enterPayload = JSON.parse(route.request().postData() || '{}');
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          results: [
            {
              listing_id: 'lst-1',
              title: 'US Grocery Price Index',
              description: 'Weekly national grocery pricing',
              category: 'pricing',
              seller_name: 'RetailMetrics',
              price_cents: 250,
              score: 0.8,
            },
          ],
          total: 1,
          mode: 'text',
        }),
      });
    });

    await page.goto('/?e2e=1');

    await page.locator('#chat-input').fill('grocery data');
    await page.locator('#chat-send').click();

    await expect(page.getByText('grocery data')).toBeVisible();
    await expect(
      page.getByText('What region should the data cover?')
    ).toBeVisible({ timeout: 10_000 });

    await page.locator('#chat-input').fill('United States, last 12 months');
    await page.locator('#chat-send').click();

    await expect(
      page.getByText(/Got it\. Searching the Exchange for: Track US grocery prices weekly/)
    ).toBeVisible({ timeout: 10_000 });

    // The harness enter should have been called with a query that includes
    // the briefing's goal_summary + selection_criteria — that's the only way
    // the buyer's clarified intent crosses into the search. runFlow plays the
    // agent-runs-to-building animation (~2-3s) before calling /agent/enter,
    // so allow a generous polling window.
    await expect.poll(() => enterPayload, { timeout: 15_000 }).not.toBeNull();
    expect(enterPayload.query).toContain('Track US grocery prices weekly');
    expect(enterPayload.query).toContain('US');
    expect(enterPayload.query).toContain('weekly cadence');

    await expect(page.getByText('US Grocery Price Index')).toBeVisible({
      timeout: 15_000,
    });

    expect(respondCallCount).toBe(1);
  });

  test('prepper 503 drops to direct search without blocking the flow', async ({ page }) => {
    let prepperHit = false;
    await page.route('**/api/prepper/**', async (route) => {
      prepperHit = true;
      await route.fulfill({ status: 503, body: '' });
    });

    await page.route(/\/agent\/enter$/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          results: [
            {
              listing_id: 'lst-1',
              title: 'Fallback Listing',
              description: 'seen when prepper is unavailable',
              category: 'pricing',
              seller_name: 'TestSeller',
              price_cents: 199,
              score: 0.5,
            },
          ],
          total: 1,
          mode: 'text',
        }),
      });
    });

    await page.goto('/?e2e=1');

    await page.locator('#chat-input').fill('pricing data for electronics');
    await page.locator('#chat-send').click();

    await expect(page.getByText(/I found several relevant sources/i)).toBeVisible({
      timeout: 15_000,
    });
    await expect(page.getByText('Fallback Listing')).toBeVisible();
    expect(prepperHit).toBe(true);
  });
});
