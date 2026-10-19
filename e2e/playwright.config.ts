import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  retries: process.env.CI ? 1 : 0,
  use: {
    baseURL: process.env.E2E_BASE_URL,
  },
  projects: [{ name: 'firefox', use: { ...devices['Desktop Firefox'] } }],
});
