import { describe, it, expect } from "vitest";
import { buildCsv, csvCell } from "@/lib/csv/export";

describe("csv export", () => {
  it("neutralizes formula-leading cells (threat model §10/§14)", () => {
    expect(csvCell("=SUM(A1)")).toBe("'=SUM(A1)");
    expect(csvCell("+1")).toBe("'+1");
    expect(csvCell("-1")).toBe("'-1");
    expect(csvCell("@cmd")).toBe("'@cmd");
    expect(csvCell("\tx")).toBe("'\tx");
  });

  it("renders nulls as empty cells, never 0/null", () => {
    expect(csvCell(null)).toBe("");
    expect(csvCell(undefined)).toBe("");
    expect(csvCell(0)).toBe("0");
    expect(csvCell(1.22)).toBe("1.22");
  });

  it("quotes cells containing commas or quotes", () => {
    expect(csvCell("a,b")).toBe('"a,b"');
    expect(csvCell('a"b')).toBe('"a""b"');
  });

  it("emits the binding header block, blank line, headers, then rows", () => {
    const csv = buildCsv({
      screen: "Trends (S-04)",
      methodologyVersion: "2026.1",
      weightsVersion: "w-2026.1",
      attribution: [{ provider: "Open-Meteo", url: "https://open-meteo.com" }],
      columns: ["provider", "mae_c"],
      rows: [
        ["Open-Meteo", 1.22],
        ["Gap", null],
      ],
      generatedAt: "2026-07-22T10:00:00Z",
    });
    const lines = csv.split("\n");
    expect(lines[0]).toBe("# ForecastIQ Export");
    expect(csv).toContain("# Generated: 2026-07-22T10:00:00Z");
    expect(csv).toContain("# Methodology: 2026.1");
    expect(csv).toContain("# Weights: w-2026.1");
    expect(csv).toContain("# Attribution: Open-Meteo — https://open-meteo.com");
    expect(csv).toContain("# Disclaimer: ForecastIQ measures forecast accuracy. We don't deliver weather forecasts.");

    const headerIdx = lines.indexOf("provider,mae_c");
    expect(headerIdx).toBeGreaterThan(0);
    expect(lines[headerIdx - 1]).toBe(""); // blank line before headers
    expect(lines[headerIdx + 1]).toBe("Open-Meteo,1.22");
    expect(lines[headerIdx + 2]).toBe("Gap,"); // null → empty cell
  });
});
