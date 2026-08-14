import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";
import { loginAsOwner } from "./helpers/session";

// Every business route reachable from the shell. The happy-path spec checks a
// handful of screens in the middle of a user journey; this sweep is the gate
// that keeps the remaining pages from regressing unnoticed.
const routes = [
  "/",
  "/customers",
  "/scanner",
  "/loyalty",
  "/referrals",
  "/campaigns",
  "/analytics",
  "/reports",
  "/integrations",
  "/risk-center",
  "/employees",
  "/settings",
  "/branches",
  "/devices",
  "/bookings",
  "/notifications",
  "/reviews",
  "/website",
  "/files",
  "/api-keys",
  "/audit",
  "/modules",
  "/subscription",
  "/onboarding",
] as const;

test("every business page is free of WCAG A/AA violations", async ({ page }) => {
  test.slow();
  await loginAsOwner(page);

  const failures: string[] = [];
  for (const route of routes) {
    await page.goto(route);
    // The shell renders immediately and fills in from the API; wait for the
    // network to settle so axe sees the same DOM a user would.
    await page.waitForLoadState("networkidle");

    const results = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
      .analyze();

    for (const violation of results.violations) {
      // Include the first node's own explanation: for contrast failures it
      // carries the measured ratio and the two colours, which is what you
      // actually need to fix it.
      const detail = violation.nodes
        .slice(0, 3)
        .map(node => `    ${node.target.join(" ")}\n      ${node.any[0]?.message ?? ""}`)
        .join("\n");
      failures.push(
        `${route} — ${violation.id} (${violation.impact}, ${violation.nodes.length} шт.): ${violation.help}\n${detail}`,
      );
    }
  }

  expect(failures, `\n${failures.join("\n")}\n`).toEqual([]);
});
