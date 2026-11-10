import { defineConfig } from "@playwright/test";

// End-to-end tests: real Chromium, virtual WebAuthn authenticators, the real Go identity
// service (dev mode) and the production Next.js build. Every test browser shares one address
// (127.0.0.1), so the per-address rate limit is raised for the test run only. The moderator
// console is routed (OPS_CONSOLE=1) with dev staff tokens.
const chromium = process.env.CHROMIUM_PATH; // e.g. /opt/pw-browsers/chromium-1194/chrome-linux/chrome

// E2E_BASE_URL=http://localhost:3000 runs against an already-running stack (for example
// deploy/gcp/compose.yml) instead of starting the services.
const external = process.env.E2E_BASE_URL;

export default defineConfig({
  testDir: "e2e",
  // Screenshot capture runs only on request: SCREENSHOTS=1 npx playwright test screens
  testIgnore: process.env.SCREENSHOTS ? [] : ["**/screens.spec.ts"],
  timeout: 60_000,
  retries: 0,
  use: {
    baseURL: external ?? "http://localhost:3000",
    launchOptions: chromium ? { executablePath: chromium } : {},
  },
  webServer: external ? [] : [
    {
      command: "cd ../backend && go build -o ../frontend/.e2e-identity-svc ./cmd/identity-svc && TRYST_ENV=dev RP_ID=localhost RP_ORIGINS=http://localhost:3000 PORT=8081 RATE_LIMIT_PER_MIN=2000 STAFF_DEV_TOKENS=mod1:moderator:e2e-mod-one-token-0001,mod2:moderator:e2e-mod-two-token-0002,lead1:lead:e2e-lead-token-00000003 ../frontend/.e2e-identity-svc",
      url: "http://localhost:8081/healthz",
      timeout: 180_000,
      reuseExistingServer: false,
    },
    {
      // The standalone safety service (D-59), talking to the API over signed internal calls.
      command: "cd ../backend && go build -o ../frontend/.e2e-safety-svc ./cmd/safety-svc && TRYST_ENV=dev PORT=8082 STAFF_DEV_TOKENS=mod1:moderator:e2e-mod-one-token-0001,mod2:moderator:e2e-mod-two-token-0002,lead1:lead:e2e-lead-token-00000003 ../frontend/.e2e-safety-svc",
      url: "http://localhost:8082/healthz",
      timeout: 180_000,
      reuseExistingServer: false,
    },
    {
      // Growth intelligence (D-73): staff API on 8083, guide pages on 8084, counts from the API.
      command: "cd ../backend && go build -o ../frontend/.e2e-growth-svc ./cmd/growth-svc && TRYST_ENV=dev PORT=8083 PUBLIC_ADDR=:8084 SITE_URL=http://localhost:8084 APP_URL=http://localhost:3000 STAFF_DEV_TOKENS=gr1:growth:e2e-growth-analyst-0001,ed1:editor:e2e-growth-editor-0001,mod1:moderator:e2e-mod-one-token-0001 ../frontend/.e2e-growth-svc",
      url: "http://localhost:8083/healthz",
      timeout: 180_000,
      reuseExistingServer: false,
    },
    {
      command: "OPS_CONSOLE=1 npm run build && TRYST_ENV=dev OPS_CONSOLE=1 npm run start -- -p 3000",
      url: "http://localhost:3000",
      timeout: 300_000,
      reuseExistingServer: false,
    },
  ],
});
