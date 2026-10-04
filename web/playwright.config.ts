import { defineConfig } from "@playwright/test";

// End-to-end passkey tests: real Chromium, virtual WebAuthn authenticators, the real Go
// identity service (dev mode) and the production Next.js build.
const chromium = process.env.CHROMIUM_PATH; // e.g. /opt/pw-browsers/chromium-1194/chrome-linux/chrome

export default defineConfig({
  testDir: "e2e",
  // Screenshot capture runs only on request: SCREENSHOTS=1 npx playwright test screens
  testIgnore: process.env.SCREENSHOTS ? [] : ["**/screens.spec.ts"],
  timeout: 60_000,
  retries: 0,
  use: {
    baseURL: "http://localhost:3000",
    launchOptions: chromium ? { executablePath: chromium } : {},
  },
  webServer: [
    {
      command: "cd ../backend && go build -o ../web/.e2e-identity-svc ./cmd/identity-svc && TRYST_ENV=dev RP_ID=localhost RP_ORIGINS=http://localhost:3000 PORT=8081 ../web/.e2e-identity-svc",
      url: "http://localhost:8081/healthz",
      timeout: 180_000,
      reuseExistingServer: false,
    },
    {
      command: "npm run build && npm run start -- -p 3000",
      url: "http://localhost:3000",
      timeout: 300_000,
      reuseExistingServer: false,
    },
  ],
});
