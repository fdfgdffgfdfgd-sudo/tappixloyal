// Visual baseline for stylesheet work. Run manually, not in CI: screenshots are
// tied to the platform that captured them, and CI has no committed baseline.
//
//   SHOT_DIR=shots-before npx playwright test tests/e2e/visual-baseline.spec.ts
//   …make the change, rebuild the web container…
//   SHOT_DIR=shots-after  npx playwright test tests/e2e/visual-baseline.spec.ts
//   node tests/e2e/compare-shots.js
//
// A change that only removes unreachable CSS should leave every file identical.
import { test } from "@playwright/test";
import { loginAsOwner } from "./helpers/session";

const routes = [
  "/", "/customers", "/scanner", "/loyalty", "/referrals", "/campaigns",
  "/analytics", "/reports", "/integrations", "/risk-center", "/employees",
  "/settings", "/branches", "/devices", "/bookings", "/notifications",
  "/reviews", "/website", "/files", "/api-keys", "/audit", "/modules",
  "/subscription", "/onboarding",
];

const dir = process.env.SHOT_DIR ?? "shots";

test("capture every screen", async ({ page }, testInfo) => {
  test.slow();
  const shot = (name: string) =>
    page.screenshot({ path: `${dir}/${testInfo.project.name}/${name}.png`, fullPage: true });

  await page.goto("/customer");
  await page.waitForLoadState("networkidle");
  await shot("_customer");

  await loginAsOwner(page);
  for (const route of routes) {
    await page.goto(route);
    await page.waitForLoadState("networkidle");
    await shot(route.replace(/\//g, "_") || "_root");
  }
});
