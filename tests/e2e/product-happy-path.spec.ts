import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

async function expectNoSeriousAccessibilityViolations(page: Page) {
  const results = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa"]).analyze();
  const blocking = results.violations.filter(item => item.impact === "critical" || item.impact === "serious");
  expect(blocking, blocking.map(item => `${item.id}: ${item.help}`).join("\n")).toEqual([]);
}

test("tenant owner can enter the workspace and navigate core tasks", async ({ page }, testInfo) => {
  const mobile = testInfo.project.name.startsWith("mobile");
  await page.goto("/login");
  await page.getByLabel("Email").fill("armat@tappix.kz");
  await page.locator('input[name="password"]').fill("Tappix2026!");
  await page.getByRole("button", { name: "Войти" }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("heading", { name: "Обзор" })).toBeVisible();

  if (mobile) await page.getByRole("button", { name: "Открыть меню" }).click();
  await page.getByRole("link", { name: "Программа", exact: true }).click();
  await expect(page).toHaveURL(/\/loyalty/);
  await expect(page.getByRole("heading", { name: "Программа лояльности" })).toBeVisible();

  if (mobile) await page.getByRole("button", { name: "Открыть меню" }).click();
  await page.getByRole("link", { name: "Отчёты", exact: true }).click();
  await expect(page).toHaveURL(/\/reports/);
  await expect(page.getByRole("heading", { name: "Регулярные отчёты" })).toBeVisible();
  await expect(page.getByRole("button", { name: /Создать/ }).first()).toBeVisible();
  await expectNoSeriousAccessibilityViolations(page);

  if (mobile) await page.getByRole("button", { name: "Открыть меню" }).click();
  await page.getByRole("link", { name: "Staff Mode", exact: true }).click();
  await expect(page).toHaveURL(/\/scanner/);
  await expect(page.getByRole("heading", { name: "Сканер гостя" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Включить камеру" })).toBeEnabled();
  await expect(page.getByText("Нет активного филиала")).toHaveCount(0);
  await page.getByLabel("Введите код клиента").fill("025 392");
  await page.getByRole("button", { name: "Найти клиента" }).click();
  await expect(page.getByRole("heading", { name: "Мадина Тест" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Отметить посещение" })).toBeEnabled();
  await expect(page.getByRole("button", { name: "Начислить" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Списать" })).toBeVisible();
  await expectNoSeriousAccessibilityViolations(page);
  const hasHorizontalOverflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
  expect(hasHorizontalOverflow).toBe(false);
});

test("customer opens the wallet and shows cashier QR with fallback code", async ({ page }) => {
  await page.goto("/customer");
  await page.getByRole("button", { name: "Войти по резервному коду" }).click();
  await page.getByLabel("Телефон").fill("+7 700 333 33 33");
  await page.getByLabel("Резервный код").fill("1234");
  await page.getByRole("button", { name: "Открыть карту" }).click();

  await expect(page.getByRole("button", { name: /Показать карту на кассе/ })).toBeVisible();
  await page.getByRole("button", { name: /Показать карту на кассе/ }).click();
  const dialog = page.getByRole("dialog", { name: "Покажите этот код сотруднику" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText("КОД КЛИЕНТА")).toBeVisible();
  await expect(dialog.getByText(/^\d{3} \d{3}$/)).toBeVisible();
  const copyCode = dialog.getByRole("button", { name: /Скопировать код/ });
  const closeDialog = dialog.getByRole("button", { name: "Закрыть" });
  await expect(copyCode).toBeVisible();
  await expect(closeDialog).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(copyCode).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(closeDialog).toBeFocused();

  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
  await expect(page.getByRole("button", { name: /Показать карту на кассе/ })).toBeFocused();
  await expectNoSeriousAccessibilityViolations(page);
  const hasHorizontalOverflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth);
  expect(hasHorizontalOverflow).toBe(false);
});
