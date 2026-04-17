import { test, expect } from '@playwright/test';

// The scripted-overlay demo is gone — the frontend now hits a real backend.
// These tests mock the HTTP surface the demo-flow calls (prepper, harness
// enter, purchases, buy-orders) so they stay deterministic and isolated
// from whether the docker-compose stack is actually running.

async function stubPrepperDisabled(page) {
  // Returning 503 on prepper start short-circuits the clarification loop
  // and drops straight into the search flow — keeps these tests focused.
  await page.route('**/api/prepper/**', async (route) => {
    await route.fulfill({ status: 503, body: '' });
  });
}

test.describe('Information Exchange demo flow', () => {
  test('shows welcome message on load', async ({ page }) => {
    await stubPrepperDisabled(page);
    await page.goto('/?e2e=1');
    await expect(
      page.getByText(/Welcome to The Information Exchange/i)
    ).toBeVisible();
  });

  test('happy path: query returns purchasable results', async ({ page }) => {
    await stubPrepperDisabled(page);

    await page.route(/\/agent\/enter$/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          results: [
            {
              listing_id: 'lst-abc',
              title: 'Consumer Electronics Pricing Index Q1 2026',
              description: 'Aggregated pricing data across 12 major retailers',
              category: 'pricing',
              seller_name: 'RetailMetrics Inc.',
              price_cents: 250,
              score: 0.85,
            },
          ],
          total: 1,
          mode: 'text',
        }),
      });
    });

    await page.route(/\/api\/v1\/purchases$/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          transaction_id: 'txn-12345678abcdef',
          already_owned: false,
        }),
      });
    });

    await page.route(/\/api\/v1\/purchases\/[^/]+\/confirm$/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          transaction_id: 'txn-12345678abcdef',
        }),
      });
    });

    await page.goto('/?e2e=1');

    await page.locator('#chat-input').fill('pricing data for electronics');
    await page.locator('#chat-send').click();

    await expect(page.getByText('pricing data for electronics')).toBeVisible();
    await expect(
      page.getByText(/I found several relevant sources/i)
    ).toBeVisible({ timeout: 15_000 });

    const buyButtons = page.getByRole('button', { name: /^Buy - \$/ });
    await expect(buyButtons.first()).toBeVisible();

    await buyButtons.first().click();
    await expect(page.getByText(/Purchase Confirmed/i)).toBeVisible();

    await page.getByRole('button', { name: 'Done' }).click();
    await expect(page.getByText(/Purchase Confirmed/i)).not.toBeVisible();
  });

  test('no-results path: agent offers a buy request form', async ({ page }) => {
    await stubPrepperDisabled(page);

    await page.route(/\/agent\/enter$/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ results: [], total: 0, mode: 'text' }),
      });
    });

    await page.route(/\/api\/v1\/buy-orders$/, async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ id: 'bo-1' }),
      });
    });

    await page.goto('/?e2e=1');

    await page.locator('#chat-input').fill('obscure niche dataset');
    await page.locator('#chat-send').click();

    await expect(
      page.getByText(/recommend posting a buy request/i)
    ).toBeVisible({ timeout: 15_000 });

    const titleInput = page.locator('.br-title');
    await expect(titleInput).toBeVisible();
    await expect(titleInput).toHaveValue(/obscure niche dataset/);

    await page.getByRole('button', { name: 'Post Buy Request' }).click();
    await expect(page.getByText(/Buy Request Posted/i)).toBeVisible();

    await page.getByRole('button', { name: 'Done' }).click();
    await expect(page.getByText(/Buy Request Posted/i)).not.toBeVisible();
  });
});
