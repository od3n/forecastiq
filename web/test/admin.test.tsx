import { render } from "@testing-library/react";
import { axe } from "jest-axe";
import { describe, it, expect, vi } from "vitest";
import { HealthGrid, type HealthCell } from "@/components/HealthGrid";
import { ProviderAdminTable, type ProviderEntry } from "@/components/ProviderAdminTable";
import { LocationAdminTable, type LocationEntry } from "@/components/LocationAdminTable";
import { ScheduleTable, type ScheduleEntry } from "@/components/ScheduleTable";
import { UserAdminTable, type UserEntry } from "@/components/UserAdminTable";

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

const LOCATIONS: LocationEntry[] = [
  { id: "l1", name: "NYC", country_code: "US", latitude: 40.7128, longitude: -74.006, timezone: "America/New_York", status: "active" },
  { id: "l2", name: "London", country_code: "GB", latitude: 51.5074, longitude: -0.1278, timezone: "Europe/London", status: "deactivated" },
];

const SCHEDULES: ScheduleEntry[] = [
  { id: "s1", slot_type: "forecast_collection", provider_name: "Open-Meteo", location_name: "NYC", cron: "10 * * * *", status: "active", last_run: "2026-07-24T12:10:00Z", next_run: "2026-07-24T13:10:00Z" },
  { id: "s2", slot_type: "analysis_batch", provider_name: null, location_name: null, cron: "10,40 * * * *", status: "paused", last_run: null, next_run: null },
];

const USERS: UserEntry[] = [
  { id: "u1", email: "admin@example.com", role: "admin", status: "active", created_at: "2026-01-01T00:00:00Z", last_login_at: "2026-07-24T00:00:00Z" },
  { id: "u2", email: "user@example.com", role: "user", status: "disabled", created_at: "2026-06-01T00:00:00Z", last_login_at: null },
];

describe("a11y: S-12 LocationAdminTable", () => {
  it("has no axe violations", async () => {
    const { container } = render(<LocationAdminTable locations={LOCATIONS} onToggleStatus={vi.fn()} onCreate={vi.fn()} />);
    const results = await axe(container, { rules: { region: { enabled: false }, "landmark-one-main": { enabled: false }, "page-has-heading-one": { enabled: false } } });
    expect(results).toHaveNoViolations();
  });

  it("shows status text", () => {
    const { getByText } = render(<LocationAdminTable locations={LOCATIONS} onToggleStatus={vi.fn()} onCreate={vi.fn()} />);
    expect(getByText("active")).toBeTruthy();
    expect(getByText("deactivated")).toBeTruthy();
  });
});

describe("a11y: S-13 ScheduleTable", () => {
  it("has no axe violations", async () => {
    const { container } = render(<ScheduleTable schedules={SCHEDULES} onToggle={vi.fn()} />);
    const results = await axe(container, { rules: { region: { enabled: false }, "landmark-one-main": { enabled: false }, "page-has-heading-one": { enabled: false } } });
    expect(results).toHaveNoViolations();
  });
});

describe("a11y: S-14 UserAdminTable", () => {
  it("has no axe violations", async () => {
    const { container } = render(<UserAdminTable users={USERS} onRoleChange={vi.fn()} onToggleStatus={vi.fn()} onExport={vi.fn()} />);
    const results = await axe(container, { rules: { region: { enabled: false }, "landmark-one-main": { enabled: false }, "page-has-heading-one": { enabled: false } } });
    expect(results).toHaveNoViolations();
  });

  it("shows role and status text", () => {
    const { getByText } = render(<UserAdminTable users={USERS} onRoleChange={vi.fn()} onToggleStatus={vi.fn()} onExport={vi.fn()} />);
    expect(getByText("active")).toBeTruthy();
    expect(getByText("disabled")).toBeTruthy();
  });
});
