import { describe, expect, test } from "vitest";
import {
  accountStatusLabel,
  companyStatusLabel,
  customerLevelLabel,
  deliveryStatusLabel,
  fileKindLabel,
  subscriptionStatusLabel,
  workspaceRoleLabel,
} from "./labels";

describe("customerLevelLabel", () => {
  test("translates the levels the API stores", () => {
    expect(customerLevelLabel("basic")).toBe("Базовый");
    expect(customerLevelLabel("silver")).toBe("Серебряный");
    expect(customerLevelLabel("gold")).toBe("Золотой");
  });

  test("keeps VIP, which reads the same in Russian", () => {
    expect(customerLevelLabel("vip")).toBe("VIP");
  });

  test("passes an unknown level through so it is visible in review", () => {
    expect(customerLevelLabel("diamond")).toBe("diamond");
  });

  test("renders nothing for a missing value", () => {
    expect(customerLevelLabel(undefined)).toBe("");
    expect(customerLevelLabel(null)).toBe("");
    expect(customerLevelLabel("")).toBe("");
  });
});

describe("subscriptionStatusLabel", () => {
  test("translates the statuses a subscription can hold", () => {
    expect(subscriptionStatusLabel("active")).toBe("Активна");
    expect(subscriptionStatusLabel("trial")).toBe("Пробный период");
  });

  test("matches regardless of the case the API used", () => {
    expect(subscriptionStatusLabel("ACTIVE")).toBe("Активна");
  });
});

describe("workspaceRoleLabel", () => {
  test("translates membership and account roles", () => {
    expect(workspaceRoleLabel("owner")).toBe("Владелец");
    expect(workspaceRoleLabel("company_owner")).toBe("Владелец");
    expect(workspaceRoleLabel("employee")).toBe("Сотрудник");
  });
});

describe("platform console states", () => {
  test("agrees with the noun it describes", () => {
    // компания активна, пользователь активен — same enum, different wording.
    expect(companyStatusLabel("active")).toBe("Активна");
    expect(accountStatusLabel("active")).toBe("Активен");
    expect(companyStatusLabel("archived")).toBe("Скрыта");
    expect(accountStatusLabel("archived")).toBe("Скрыт");
  });

  test("covers the states the API can set", () => {
    expect(companyStatusLabel("blocked")).toBe("Заблокирована");
    expect(accountStatusLabel("invited")).toBe("Приглашён");
  });
});

describe("delivery and file labels", () => {
  test("translates what the notifications table shows", () => {
    expect(deliveryStatusLabel("sent")).toBe("Отправлено");
    expect(deliveryStatusLabel("failed")).toBe("Ошибка");
  });

  test("matches the wording already used in the upload form", () => {
    expect(fileKindLabel("logo")).toBe("Логотип");
    expect(fileKindLabel("asset")).toBe("Изображение");
    expect(fileKindLabel("document")).toBe("Документ");
  });
});
