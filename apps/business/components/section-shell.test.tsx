import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, test, vi } from "vitest";

const api = vi.fn();
vi.mock("@/lib/api", () => ({ api: (path: string) => api(path), logout: vi.fn() }));

import { SectionShell } from "./section-shell";

function respondWith(identity: unknown, workspaces: unknown[] = []) {
  api.mockImplementation((path: string) => {
    if (path === "/auth/me") return Promise.resolve(identity);
    if (path === "/workspaces") return Promise.resolve(workspaces);
    return Promise.reject(new Error(`unexpected ${path}`));
  });
}

function renderShell() {
  return render(
    <SectionShell active="/" title="Обзор" subtitle="Сегодня">
      <p>содержимое</p>
    </SectionShell>,
  );
}

describe("SectionShell", () => {
  beforeEach(() => {
    api.mockReset();
  });

  test("shows the person who is signed in", async () => {
    respondWith({ role: "company_owner", firstName: "Мадина", lastName: "Ким" });
    renderShell();

    expect(await screen.findByText("Мадина Ким")).toBeInTheDocument();
    expect(screen.getByText("Владелец")).toBeInTheDocument();
  });

  test("never invents an identity when the profile cannot be loaded", async () => {
    api.mockRejectedValue(new Error("сеть недоступна"));
    renderShell();

    // The header used to fall back to a hardcoded name, so every operator was
    // greeted as somebody else.
    await waitFor(() => expect(screen.getByText("Нет связи с сервером")).toBeInTheDocument());
    expect(screen.queryByText("Армат")).not.toBeInTheDocument();
    expect(screen.queryByText("Владелец")).not.toBeInTheDocument();
    expect(screen.queryByText("Сотрудник")).not.toBeInTheDocument();
  });

  test("reports the connection instead of claiming the system is fine", async () => {
    respondWith({ role: "company_owner", firstName: "Мадина", lastName: "Ким" });
    renderShell();

    expect(screen.getByText("Проверяем соединение…")).toBeInTheDocument();
    expect(await screen.findByText("Система работает")).toBeInTheDocument();
  });

  test("offers owner sections only once the role says owner", async () => {
    respondWith({ role: "employee", firstName: "Аслан", lastName: "Ким" });
    renderShell();

    expect(await screen.findByText("Аслан Ким")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Клиенты/ })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Настройки/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /Интеграции/ })).not.toBeInTheDocument();
  });

  test("translates the membership role in the workspace switcher", async () => {
    respondWith({ role: "company_owner", firstName: "Мадина", lastName: "Ким" }, [
      { id: "1", name: "Dentline", slug: "dentline", role: "owner", plan: "Business", current: true },
    ]);
    renderShell();

    await userEvent.click(await screen.findByRole("button", { name: /Dentline/ }));

    // "Владелец" also labels the signed-in person in the header, so look inside
    // the switcher rather than at the page.
    const menu = within(await screen.findByRole("menu"));
    expect(menu.getByText("Владелец")).toBeInTheDocument();
    expect(menu.queryByText("owner")).not.toBeInTheDocument();
  });
});
