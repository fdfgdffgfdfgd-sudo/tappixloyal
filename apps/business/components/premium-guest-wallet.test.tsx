import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { PremiumGuestWallet } from "./premium-guest-wallet";

// No session: every wallet call fails, so the component falls back to sign-in.
function signedOut() {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({ ok: false, status: 401, json: async () => ({ success: false }) }),
  );
}

function visitWith(search: string) {
  window.history.replaceState({}, "", `/customer${search}`);
}

describe("PremiumGuestWallet sign-in", () => {
  beforeEach(() => {
    localStorage.clear();
    visitWith("");
    signedOut();
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  test("asks which company the card belongs to when nothing identifies it", async () => {
    render(<PremiumGuestWallet />);

    const field = await screen.findByLabelText(/Компания/);
    expect(field).toBeVisible();
    expect(field).toHaveValue("");
  });

  test("takes the company from the link", async () => {
    visitWith("?company=docmed");
    render(<PremiumGuestWallet />);

    await waitFor(() =>
      expect(document.querySelector('input[name="company"]')).toHaveValue("docmed"),
    );
    // Known tenant: nothing to ask the guest about.
    expect(screen.queryByLabelText(/Компания/)).not.toBeInTheDocument();
  });

  test("remembers the company from a previous visit", async () => {
    localStorage.setItem("tappix_guest_company", "docmed");
    render(<PremiumGuestWallet />);

    await waitFor(() =>
      expect(document.querySelector('input[name="company"]')).toHaveValue("docmed"),
    );
  });

  test("never falls back to a particular tenant", async () => {
    render(<PremiumGuestWallet />);

    await screen.findByLabelText(/Компания/);
    expect(document.querySelector('input[name="company"]')).not.toHaveValue("dentline");
  });
});
