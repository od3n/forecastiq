import { render, fireEvent } from "@testing-library/react";
import { axe } from "jest-axe";
import { describe, it, expect } from "vitest";
import { RankingTable, type RankingEntry } from "@/components/RankingTable";
import { StatusBadge } from "@/components/StatusBadge";
import { MetricTable } from "@/components/MetricTable";
import { HorizonSelector } from "@/components/HorizonSelector";

const SAMPLE_RANKINGS: RankingEntry[] = [
  { rank: 1, provider_id: "a", provider_name: "Open-Meteo", composite_score: 0.94, ranking_status: "ranked", sample_count: 720, coverage: 0.98, components: { mae: 0.12, rmse: 0.15 }, penalty_applied: false },
  { rank: null, provider_id: "b", provider_name: "OW", composite_score: null, ranking_status: "unranked", sample_count: 10, coverage: null },
];

describe("a11y: S-01/S-02 components", () => {
  it("RankingTable has no axe violations", async () => {
    const { container } = render(
      <RankingTable rankings={SAMPLE_RANKINGS} freshness={{ state: "fresh", last_updated: "2026-07-24T00:00:00Z" }} methodologyVersion="2026.1" />,
    );
    const results = await axe(container, { rules: { region: { enabled: false }, "landmark-one-main": { enabled: false }, "page-has-heading-one": { enabled: false } } });
    expect(results).toHaveNoViolations();
  });

  it("StatusBadge renders accessible text for all statuses", () => {
    const { getByText, rerender } = render(<StatusBadge status="ranked" />);
    expect(getByText("Ranked")).toBeTruthy();
    rerender(<StatusBadge status="provisionally_ranked" sampleCount={15} />);
    expect(getByText(/Provisional/)).toBeTruthy();
    // ≥ 10 pairs with no coverage value: coverage is provably the trigger
    // (backend treats nil coverage as 0) — never the wrong-reason sample copy.
    rerender(<StatusBadge status="unranked" sampleCount={10} />);
    expect(getByText("Insufficient coverage")).toBeTruthy();
    // Coverage-triggered unranked (BR-RANK-04): samples are fine, coverage < 0.5.
    rerender(<StatusBadge status="unranked" sampleCount={43} coverage={0.42} />);
    expect(getByText("Insufficient coverage (42% / 50% required)")).toBeTruthy();
    // Floor, not round: 0.499 reads 49%, never a self-contradictory 50%.
    rerender(<StatusBadge status="unranked" sampleCount={43} coverage={0.499} />);
    expect(getByText("Insufficient coverage (49% / 50% required)")).toBeTruthy();
    // Coverage exactly at the floor is not below it: falls back to sample copy.
    rerender(<StatusBadge status="unranked" sampleCount={43} coverage={0.5} />);
    expect(getByText(/Insufficient data.*43\/30/)).toBeTruthy();
    // Sample-driven unranked keeps the sample copy even when coverage is low.
    rerender(<StatusBadge status="unranked" sampleCount={5} coverage={0.3} />);
    expect(getByText(/Insufficient data.*5\/30/)).toBeTruthy();
  });

  it("MetricTable has no axe violations", async () => {
    const { container } = render(
      <MetricTable variable="temperature" unit="°C" rows={[{ provider_id: "a", provider_name: "OM", mae: 1.22, rmse: 1.5, bias: 0.3, sample_count: 100 }]} />,
    );
    const results = await axe(container, { rules: { region: { enabled: false }, "landmark-one-main": { enabled: false }, "page-has-heading-one": { enabled: false } } });
    expect(results).toHaveNoViolations();
  });

  it("RankingTable row expands on Enter key", () => {
    const { container, getByText } = render(<RankingTable rankings={SAMPLE_RANKINGS} />);
    const btn = container.querySelector("button[aria-expanded]") as HTMLElement;
    fireEvent.keyDown(btn, { key: "Enter" });
    expect(getByText("Component breakdown")).toBeTruthy();
  });

  it("RankingTable shows sample-driven copy when samples AND coverage are both low", () => {
    // Mirrors the API row for a provider with 5 pairs + coverage 0.30: the
    // sample floor is the trigger, so the coverage message must not appear.
    const rows: RankingEntry[] = [
      SAMPLE_RANKINGS[0],
      { rank: null, provider_id: "c", provider_name: "PX", composite_score: null, ranking_status: "unranked", sample_count: 5, coverage: 0.3 },
    ];
    const { getByText, queryByText } = render(<RankingTable rankings={rows} />);
    expect(getByText("Insufficient data (5/30)")).toBeTruthy();
    expect(queryByText(/Insufficient coverage/)).toBeNull();
  });
});

describe("HorizonSelector interaction", () => {
  it("calls onChange with the selected minutes", () => {
    let selected = 1440;
    const onChange = (m: number) => { selected = m; };
    const { getByText } = render(<HorizonSelector selected={1440} onChange={onChange} />);
    fireEvent.click(getByText("+3h"));
    expect(selected).toBe(180);
  });

  it("marks the active pill with aria-checked", () => {
    const { getByText } = render(<HorizonSelector selected={1440} onChange={() => {}} />);
    expect(getByText("+24h").getAttribute("aria-checked")).toBe("true");
    expect(getByText("+1h").getAttribute("aria-checked")).toBe("false");
  });
});
