import { render } from "@testing-library/react";
import { axe } from "jest-axe";
import { describe, it, expect, vi } from "vitest";
import { HealthGrid, type HealthCell } from "@/components/HealthGrid";
import { ProviderAdminTable, type ProviderEntry } from "@/components/ProviderAdminTable";

const HEALTH_CELLS: HealthCell[] = [
  { provider_name: "Open-Meteo", location_name: "NYC", last_success: "2026-07-24T12:00:00Z", status: "ok", freshness: "fresh", circuit_state: "closed", next_scheduled_at: "2026-07-24T13:10:00Z", provider_id: "a", location_id: "b" },
  { provider_name: "OpenWeather", location_name: "LDN", last_success: null, status: "stale", freshness: "stale", circuit_state: "open", next_scheduled_at: null, provider_id: "c", location_id: "d" },
];

const PROVIDERS: ProviderEntry[] = [
  { id: "a", name: "Open-Meteo", slug: "open-meteo", status: "active", has_credential: true, minute_offset: 10 },
  { id: "b", name: "OpenWeather", slug: "openweather", status: "disabled", has_credential: false, minute_offset: 5 },
];

describe("a11y: S-10 HealthGrid", () => {
  it("has no axe violations", async () => {
    const { container } = render(<HealthGrid cells={HEALTH_CELLS} onRetry={vi.fn()} />);
    const results = await axe(container, { rules: { region: { enabled: false }, "landmark-one-main": { enabled: false }, "page-has-heading-one": { enabled: false } } });
    expect(results).toHaveNoViolations();
  });

  it("retry button has specific aria-label", () => {
    const { getAllByRole } = render(<HealthGrid cells={HEALTH_CELLS} onRetry={vi.fn()} />);
    const btns = getAllByRole("button");
    expect(btns[0].getAttribute("aria-label")).toBe("Re-collect now for Open-Meteo at NYC");
  });
});

describe("a11y: S-11 ProviderAdminTable", () => {
  it("has no axe violations", async () => {
    const { container } = render(<ProviderAdminTable providers={PROVIDERS} onToggleStatus={vi.fn()} onEditConfig={vi.fn()} />);
    const results = await axe(container, { rules: { region: { enabled: false }, "landmark-one-main": { enabled: false }, "page-has-heading-one": { enabled: false } } });
    expect(results).toHaveNoViolations();
  });

  it("shows status text (not color-only)", () => {
    const { getByText } = render(<ProviderAdminTable providers={PROVIDERS} onToggleStatus={vi.fn()} onEditConfig={vi.fn()} />);
    expect(getByText("active")).toBeTruthy();
    expect(getByText("disabled")).toBeTruthy();
  });

  it("credential shows text status only (BR-08)", () => {
    const { getByText } = render(<ProviderAdminTable providers={PROVIDERS} onToggleStatus={vi.fn()} onEditConfig={vi.fn()} />);
    expect(getByText("Configured")).toBeTruthy();
    expect(getByText("Not set")).toBeTruthy();
  });
});
