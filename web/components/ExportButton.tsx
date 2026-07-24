"use client";

import { useCallback } from "react";
import { buildCsv, downloadCsv, type CsvExportInput } from "@/lib/csv/export";

export interface ExportButtonProps {
  /** CSV metadata for the current view. */
  exportInput: CsvExportInput | null;
  /** Filename (without extension). */
  filename?: string;
  disabled?: boolean;
  disabledReason?: string;
}

// Export CSV button (doc 02 §14.3 component inventory; conventions §5). Generates
// the CSV client-side from the current view data and triggers a download. Disabled
// with tooltip when no data is available (offline/error state).
export function ExportButton({ exportInput, filename = "forecastiq-export", disabled, disabledReason }: ExportButtonProps) {
  const handleExport = useCallback(() => {
    if (!exportInput) return;
    const csv = buildCsv(exportInput);
    downloadCsv(`${filename}.csv`, csv);
  }, [exportInput, filename]);

  return (
    <button
      type="button"
      onClick={handleExport}
      disabled={disabled || !exportInput}
      title={disabled ? disabledReason : undefined}
      style={{
        padding: "var(--space-sm) var(--space-md)",
        background: "var(--color-primary)",
        color: "#ffffff",
        border: "none",
        borderRadius: "var(--radius-md)",
        font: "inherit",
        fontWeight: 500,
        cursor: disabled || !exportInput ? "not-allowed" : "pointer",
        opacity: disabled || !exportInput ? 0.5 : 1,
        minHeight: 36,
      }}
    >
      Export CSV
    </button>
  );
}
