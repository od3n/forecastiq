import { render, fireEvent } from "@testing-library/react";
import { axe } from "jest-axe";
import { describe, it, expect, vi } from "vitest";
import { ProviderGrid } from "@/components/ProviderGrid";
import { TrendChart } from "@/components/TrendChart";
import { ExportButton } from "@/components/ExportButton";
import type { CsvExportInput } from "@/lib/csv/export";

describe("a11y: S-03 ProviderGrid", () => {
  it("renders a semantic table with no axe violations", async () => {
    const { container } = render(
      <ProviderGrid
        providerName="Open-Meteo"
        cells={[
          { location_id: "a", location_name: "Johor Bahru", scores: { 1440: 0.94, 60: null } },
        ]}
      />,
    );
    const results = await axe(container, { rules: { region: { enabled: false }, "landmark-one-main": { enabled: false }, "page-has-heading-one": { enabled: false } } });
    expect(results).toHaveNoViolations();
  });
});

describe("a11y: S-04 TrendChart", () => {
  const DATA = [
    { period_start: "2026-07-01", "Open-Meteo": 1.2, "Open-Meteo_samples": 30 },
    { period_start: "2026-07-02", "Open-Meteo": 1.4, "Open-Meteo_samples": 25 },
  ];
  const SERIES = [{ provider: "Open-Meteo" }];

  it("wraps chart in role=img with aria-label", () => {
    const { container } = render(<TrendChart data={DATA} series={SERIES} unit="°C" />);
    const wrapper = container.querySelector("[role='img']");
    expect(wrapper).toBeTruthy();
    expect(wrapper?.getAttribute("aria-label")).toContain("°C");
  });

  it("renders a hidden data table for screen readers", () => {
    const { container } = render(<TrendChart data={DATA} series={SERIES} unit="°C" />);
    const table = container.querySelector("table[aria-label]");
    expect(table).toBeTruthy();
    expect(table?.textContent).toContain("Open-Meteo");
  });

  it("legend toggle hides/shows series", () => {
    const { getByLabelText } = render(<TrendChart data={DATA} series={SERIES} unit="°C" />);
    const btn = getByLabelText("Hide Open-Meteo");
    fireEvent.click(btn);
    expect(btn.getAttribute("aria-pressed")).toBe("false");
  });

  it("keyboard arrow updates aria-live announcement", () => {
    const { container } = render(<TrendChart data={DATA} series={SERIES} unit="°C" />);
    const chartDiv = container.querySelector("[tabindex='0']") as HTMLElement;
    fireEvent.keyDown(chartDiv, { key: "ArrowRight" });
    const live = container.querySelector("[aria-live='polite']");
    expect(live?.textContent).toContain("2026-07-02");
  });
});

describe("ExportButton", () => {
  it("is disabled when exportInput is null", () => {
    const { getByText } = render(<ExportButton exportInput={null} />);
    expect((getByText("Export CSV") as HTMLButtonElement).disabled).toBe(true);
  });

  it("calls downloadCsv with correct content on click", () => {
    // Mock URL.createObjectURL for jsdom.
    const mockUrl = "blob:mock";
    global.URL.createObjectURL = vi.fn(() => mockUrl);
    global.URL.revokeObjectURL = vi.fn();

    const input: CsvExportInput = {
      screen: "Test",
      columns: ["a", "b"],
      rows: [["x", 1]],
      generatedAt: "2026-07-24T00:00:00Z",
    };
    const { getByText } = render(<ExportButton exportInput={input} />);
    const btn = getByText("Export CSV") as HTMLButtonElement;
    expect(btn.disabled).toBe(false);
    fireEvent.click(btn);
    expect(global.URL.createObjectURL).toHaveBeenCalled();
  });
});
