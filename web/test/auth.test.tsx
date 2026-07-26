import { render, fireEvent, waitFor } from "@testing-library/react";
import { axe } from "jest-axe";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  safeReturnPath,
  getDevToken,
  setDevToken,
  clearDevToken,
  getAccessToken,
  authHeaders,
} from "@/lib/auth/session";
import SignInPage from "@/app/auth/signin/page";

// The signin page reads the return target and router from next/navigation.
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  useSearchParams: () => new URLSearchParams("return=/admin/health"),
}));

describe("safeReturnPath (open-redirect guard)", () => {
  it("defaults to the Overview", () => {
    expect(safeReturnPath(null)).toBe("/");
    expect(safeReturnPath(undefined)).toBe("/");
    expect(safeReturnPath("")).toBe("/");
  });

  it("accepts same-origin absolute paths", () => {
    expect(safeReturnPath("/admin/health")).toBe("/admin/health");
    expect(safeReturnPath("/trends?horizon_minutes=1440")).toBe("/trends?horizon_minutes=1440");
  });

  it("rejects external and protocol-relative targets", () => {
    expect(safeReturnPath("https://evil.example")).toBe("/");
    expect(safeReturnPath("//evil.example")).toBe("/");
    expect(safeReturnPath("javascript:alert(1)")).toBe("/");
  });
});

describe("dev session token store (Supabase unconfigured)", () => {
  beforeEach(() => window.localStorage.clear());

  it("stores, resolves, and clears the dev token", async () => {
    expect(getDevToken()).toBeNull();
    await expect(getAccessToken()).resolves.toBeNull();

    setDevToken("admin");
    expect(getDevToken()).toBe("admin");
    await expect(getAccessToken()).resolves.toBe("admin");

    clearDevToken();
    expect(getDevToken()).toBeNull();
  });

  it("authHeaders attaches the bearer only when signed in", async () => {
    await expect(authHeaders()).resolves.toEqual({ "Content-Type": "application/json" });

    setDevToken("admin");
    await expect(authHeaders()).resolves.toEqual({
      "Content-Type": "application/json",
      Authorization: "Bearer admin",
    });
  });
});

describe("S-08 dev-mode sign in", () => {
  beforeEach(() => window.localStorage.clear());
  afterEach(() => vi.unstubAllGlobals());

  function mockMeResponse(ok: boolean, status = ok ? 200 : 401) {
    const fetchMock = vi.fn().mockResolvedValue({
      ok,
      status,
      headers: { get: () => null },
      json: async () => (ok ? { data: { user: { role: "admin" } } } : { title: "Unauthorized" }),
    });
    vi.stubGlobal("fetch", fetchMock);
    return fetchMock;
  }

  it("renders the dev token form and has no axe violations", async () => {
    const { container, getByLabelText } = render(<SignInPage />);
    expect(getByLabelText("Dev token")).toBeTruthy();
    const results = await axe(container, {
      rules: { region: { enabled: false }, "landmark-one-main": { enabled: false } },
    });
    expect(results).toHaveNoViolations();
  });

  it("stores the token only after the API accepts it", async () => {
    const fetchMock = mockMeResponse(true);
    const { getByLabelText, getByRole } = render(<SignInPage />);

    fireEvent.change(getByLabelText("Dev token"), { target: { value: "admin" } });
    fireEvent.click(getByRole("button", { name: /sign in/i }));

    await waitFor(() => expect(getDevToken()).toBe("admin"));
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain("/me");
    expect((init.headers as Record<string, string>)["Authorization"]).toBe("Bearer admin");
  });

  it("keeps the session empty and shows an error on a rejected token", async () => {
    mockMeResponse(false);
    const { getByLabelText, getByRole, findByRole } = render(<SignInPage />);

    fireEvent.change(getByLabelText("Dev token"), { target: { value: "nope" } });
    fireEvent.click(getByRole("button", { name: /sign in/i }));

    const alert = await findByRole("alert");
    expect(alert.textContent).toMatch(/Sign-in failed/);
    expect(getDevToken()).toBeNull();
  });
});
