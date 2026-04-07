import { test, expect } from '@playwright/test';

test.describe('Information Exchange demo flow', () => {
  test('shows welcome message on load', async ({ page }) => {
    await page.goto('/?e2e=1');
    await expect(
      page.getByText(/Welcome to The Information Exchange/i)
    ).toBeVisible();
  });

  test('happy path: query returns purchasable results', async ({ page }) => {
    await page.goto('/?e2e=1');

    // Force the "results" path so this test is deterministic.
    await page.locator('#demo-mode').selectOption('results');

    await page.locator('#chat-input').fill('pricing data for electronics');
    await page.locator('#chat-send').click();

    // Echoed user message
    await expect(page.getByText('pricing data for electronics')).toBeVisible();

    // Agent response with results
    await expect(
      page.getByText(/I found several relevant sources/i)
    ).toBeVisible({ timeout: 15_000 });

    // At least one Buy button appears
    const buyButtons = page.getByRole('button', { name: /^Buy - \$/ });
    await expect(buyButtons.first()).toBeVisible();

    // Click the first Buy button -> overlay confirms purchase
    await buyButtons.first().click();
    await expect(page.getByText(/Purchase Confirmed/i)).toBeVisible();

    // Dismiss overlay
    await page.getByRole('button', { name: 'Done' }).click();
    await expect(page.getByText(/Purchase Confirmed/i)).not.toBeVisible();
  });

  test('no-results path: agent offers a buy request form', async ({ page }) => {
    await page.goto('/?e2e=1');

    await page.locator('#demo-mode').selectOption('no-results');

    await page.locator('#chat-input').fill('obscure niche dataset');
    await page.locator('#chat-send').click();

    // Agent suggests posting a buy request
    await expect(
      page.getByText(/recommend posting a buy request/i)
    ).toBeVisible({ timeout: 15_000 });

    // Buy request form appears, prefilled with the query
    const titleInput = page.locator('.br-title');
    await expect(titleInput).toBeVisible();
    await expect(titleInput).toHaveValue(/obscure niche dataset/);

    // Submit and confirm overlay
    await page.getByRole('button', { name: 'Post Buy Request' }).click();
    await expect(page.getByText(/Buy Request Posted/i)).toBeVisible();

    await page.getByRole('button', { name: 'Done' }).click();
    await expect(page.getByText(/Buy Request Posted/i)).not.toBeVisible();
  });
});
