import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  testMatch: "performance.spec.ts",
  workers: 1,
  use: {
    ...devices["Desktop Chrome"],
    deviceScaleFactor: 2,
    baseURL: "http://127.0.0.1:3200",
    trace: "on",
  },
  webServer: {
    command:
      "GRIDOS_AUTH_MODE=local pnpm exec vite preview --host 127.0.0.1 --port 3200 --strictPort",
    url: "http://127.0.0.1:3200",
    reuseExistingServer: false,
  },
});
