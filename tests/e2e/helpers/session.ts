import { expect, type Page } from "@playwright/test";

// Local development fixture seeded by migration 000006. Override through the
// environment when running against a non-local environment.
const OWNER_EMAIL = process.env.E2E_OWNER_EMAIL ?? "armat@tappix.kz";
const OWNER_PASSWORD = process.env.E2E_OWNER_PASSWORD ?? "Tappix2026!";

export async function loginAsOwner(page: Page) {
  await page.goto("/login");
  await page.getByLabel("Email").fill(OWNER_EMAIL);
  await page.locator('input[name="password"]').fill(OWNER_PASSWORD);
  await page.getByRole("button", { name: "Войти" }).click();
  await expect(page).toHaveURL(/\/$/);
}
