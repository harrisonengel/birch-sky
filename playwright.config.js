import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: 'html',
  use: {
    baseURL: 'http://localhost:4317',
    trace: 'on-first-retry',
  },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  ],
  // Use a dedicated, uncommon port so we don't collide with a dev server the
  // user (or another worktree) may already be running on Vite's default port.
  webServer: {
    command: 'npm run dev -- --port 4317 --strictPort',
    url: 'http://localhost:4317',
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
