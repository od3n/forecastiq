// Client-side CSV export (docs/api/02-response-conventions.md §5; DR-05/PC-09).
// The file is a `#`-prefixed metadata block, a blank line, column headers, then
// data rows. Bounded, synchronous generation from the current view — no server
// export infrastructure (that path exists only for GDPR, AUTH-09).

// Formula-injection guard (threat model §10/§14): a cell beginning with one of
// = + - @ TAB CR can be interpreted as a formula by spreadsheet apps; prefix a
// single quote to neutralize it. Applied to headers and data cells.
const FORMULA_LEAD = /^[=+\-@\t\r]/;

/** csvCell renders one value: nulls → empty (never "0"/"null"), formula-safe, quoted. */
export function csvCell(value: string | number | null | undefined): string {
  if (value === null || value === undefined) return "";
  let s = String(value);
  if (FORMULA_LEAD.test(s)) s = `'${s}`;
  if (/[",\n\r]/.test(s)) s = `"${s.replace(/"/g, '""')}"`;
  return s;
}

/** stripNewlines keeps interpolated comment-header values on a single line. */
function commentValue(value: string): string {
  return value.replace(/[\r\n]+/g, " ").trim();
}

export interface CsvExportInput {
  /** Screen name, e.g. "Trends (S-04)". */
  screen: string;
  methodologyVersion?: string;
  weightsVersion?: string;
  period?: string;
  location?: string;
  horizon?: string;
  variable?: string;
  observationProvenance?: string;
  attribution?: { provider: string; url: string }[];
  columns: string[];
  rows: (string | number | null | undefined)[][];
  /** Overridable for deterministic tests; defaults to now (ISO UTC). */
  generatedAt?: string;
}

const DISCLAIMER =
  "ForecastIQ measures forecast accuracy. We don't deliver weather forecasts.";

/** buildCsv produces the full CSV text per the binding structure (§5). */
export function buildCsv(x: CsvExportInput): string {
  const lines: string[] = [];
  lines.push("# ForecastIQ Export");
  lines.push(`# Generated: ${x.generatedAt ?? new Date().toISOString()}`);
  lines.push(`# Screen: ${commentValue(x.screen)}`);
  if (x.methodologyVersion) lines.push(`# Methodology: ${commentValue(x.methodologyVersion)}`);
  if (x.weightsVersion) lines.push(`# Weights: ${commentValue(x.weightsVersion)}`);
  if (x.period) lines.push(`# Period: ${commentValue(x.period)}`);
  if (x.location) lines.push(`# Location: ${commentValue(x.location)}`);
  if (x.horizon) lines.push(`# Horizon: ${commentValue(x.horizon)}`);
  if (x.variable) lines.push(`# Variable: ${commentValue(x.variable)}`);
  if (x.observationProvenance)
    lines.push(`# Observation provenance: ${commentValue(x.observationProvenance)}`);
  if (x.attribution && x.attribution.length > 0) {
    lines.push(
      `# Attribution: ${x.attribution.map((a) => `${a.provider} — ${a.url}`).join("; ")}`,
    );
  }
  lines.push(`# Disclaimer: ${DISCLAIMER}`);
  lines.push("");
  lines.push(x.columns.map(csvCell).join(","));
  for (const row of x.rows) {
    lines.push(row.map(csvCell).join(","));
  }
  return lines.join("\n");
}

/** downloadCsv triggers a browser download of the CSV text (client-only). */
export function downloadCsv(filename: string, content: string): void {
  const blob = new Blob([content], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}
