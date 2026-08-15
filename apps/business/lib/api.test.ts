import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { OFFLINE_MESSAGE, api, assetUrlFrom } from "./api";

describe("assetUrlFrom", () => {
  // Uploaded files come back as absolute paths under /api/v1.
  const file = "/api/v1/public/files/abc";

  test("leaves the path alone when the API shares the origin", () => {
    // Production: nginx proxies /api/ to the API, so the path already resolves.
    expect(assetUrlFrom("/api/v1", file)).toBe(file);
  });

  test("puts the API origin in front when the API lives elsewhere", () => {
    // Development: the panel runs on :3000 and the API on :8080.
    expect(assetUrlFrom("http://localhost:8080/api/v1", file)).toBe(
      "http://localhost:8080/api/v1/public/files/abc",
    );
  });

  test("uses the origin only, not the base path", () => {
    expect(assetUrlFrom("https://api.tappix.kz/api/v1", file)).toBe(
      "https://api.tappix.kz/api/v1/public/files/abc",
    );
  });

  test("does not depend on which port the panel itself is served from", () => {
    // The previous implementation keyed off window.location.port === "3000",
    // so serving the panel on that port in production broke every file link.
    expect(assetUrlFrom("/api/v1", file)).not.toContain("localhost");
  });
});

function jsonResponse(body: unknown, status = 200) {
  // `ok` matters: the refresh retry is gated on it.
  return { status, ok: status >= 200 && status < 300, json: async () => body } as Response;
}

describe("api", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  test("returns the payload of a successful response", async () => {
    vi.mocked(fetch).mockResolvedValue(jsonResponse({ success: true, data: { id: "1" } }));
    await expect(api<{ id: string }>("/customers")).resolves.toEqual({ id: "1" });
  });

  test("reports a lost connection as a sentence, not 'Failed to fetch'", async () => {
    vi.mocked(fetch).mockRejectedValue(new TypeError("Failed to fetch"));
    await expect(api("/customers")).rejects.toThrow(OFFLINE_MESSAGE);
  });

  test("explains a response that is not JSON instead of throwing a parser error", async () => {
    vi.mocked(fetch).mockResolvedValue({
      status: 502,
      json: async () => {
        throw new SyntaxError("Unexpected token <");
      },
    } as unknown as Response);
    await expect(api("/customers")).rejects.toThrow(/Обновите страницу/);
  });

  test("surfaces the message the API sent", async () => {
    vi.mocked(fetch).mockResolvedValue(
      jsonResponse({ success: false, error: { message: "Укажите заголовок сайта" } }, 422),
    );
    await expect(api("/website")).rejects.toThrow("Укажите заголовок сайта");
  });

  test("retries once through the refresh endpoint before giving up on a 401", async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(jsonResponse({ success: false }, 401))
      .mockResolvedValueOnce(jsonResponse({ success: true }, 200)) // refresh
      .mockResolvedValueOnce(jsonResponse({ success: true, data: "ok" }));

    await expect(api<string>("/customers")).resolves.toBe("ok");
    expect(vi.mocked(fetch).mock.calls[1]?.[0]).toContain("/auth/refresh");
  });

  test("throws when the session cannot be refreshed, so callers stop", async () => {
    vi.mocked(fetch)
      .mockResolvedValueOnce(jsonResponse({ success: false }, 401))
      .mockResolvedValueOnce(jsonResponse({ success: false }, 401)) // refresh failed
      .mockResolvedValue(jsonResponse({ success: false }, 401));

    await expect(api("/customers")).rejects.toThrow(/Сессия истекла/);
  });
});
