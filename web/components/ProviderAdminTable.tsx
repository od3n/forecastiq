"use client";

import { useState } from "react";
import { ConfirmDialog } from "./ConfirmDialog";

export interface ProviderEntry {
  id: string;
  name: string;
  slug: string;
  status: string; // "active" | "disabled"
  has_credential?: boolean;
  /** If the provider is actively collecting, a credential must be configured. */
  collecting_since?: string | null;
  minute_offset: number;
}

export interface ProviderAdminTableProps {
  providers: ProviderEntry[];
  onToggleStatus: (id: string, newStatus: string) => void;
  onEditConfig: (id: string, minuteOffset: number) => void;
}

// S-11 Admin Providers table (doc 02 §4.11). Shows provider name, slug,
// status (active/disabled badge with text), credential indicator, and actions.
// Enable/disable uses ConfirmDialog; config edit is inline.
export function ProviderAdminTable({ providers, onToggleStatus, onEditConfig }: ProviderAdminTableProps) {
  const [toggling, setToggling] = useState<{ id: string; action: string } | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [editOffset, setEditOffset] = useState(0);

  return (
    <>
      <div className="tableWrap">
        <table style={{ width: "100%", borderCollapse: "collapse", minWidth: 600 }}>
          <thead>
            <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Provider</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Slug</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Status</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Credential</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>Offset (min)</th>
              <th scope="col" style={{ padding: "var(--space-sm)" }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {providers.map((p) => (
              <tr key={p.id} style={{ borderBottom: "1px solid var(--color-border)", height: 48 }}>
                <td style={{ padding: "var(--space-sm)", fontWeight: 500 }}>{p.name}</td>
                <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)" }}>{p.slug}</td>
                <td style={{ padding: "var(--space-sm)" }}>
                  <span style={{ padding: "2px var(--space-sm)", borderRadius: "var(--radius-full)", fontSize: "var(--text-label)", fontWeight: 500, background: p.status === "active" ? "var(--color-ranked)" : "var(--color-border)", color: p.status === "active" ? "#fff" : "var(--color-text-secondary)" }}>
                    {p.status}
                  </span>
                </td>
                <td style={{ padding: "var(--space-sm)", color: p.has_credential || p.collecting_since ? "var(--color-fresh)" : "var(--color-text-muted)" }}>
                  {p.has_credential || p.collecting_since ? "Configured" : "Not required"}
                </td>
                <td style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)" }}>
                  {editing === p.id ? (
                    <span style={{ display: "inline-flex", gap: "var(--space-xs)" }}>
                      <input type="number" value={editOffset} onChange={(e) => setEditOffset(Number(e.target.value))} style={{ width: 60, padding: "var(--space-xs)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", font: "inherit", textAlign: "right" }} />
                      <button type="button" onClick={() => { onEditConfig(p.id, editOffset); setEditing(null); }} style={{ padding: "var(--space-xs) var(--space-sm)", background: "var(--color-primary)", color: "#fff", border: "none", borderRadius: "var(--radius-sm)", fontSize: "var(--text-label)", cursor: "pointer" }}>Save</button>
                      <button type="button" onClick={() => setEditing(null)} style={{ padding: "var(--space-xs) var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", fontSize: "var(--text-label)", cursor: "pointer", background: "var(--color-surface)" }}>Cancel</button>
                    </span>
                  ) : (
                    <span>
                      {p.minute_offset}
                      <button type="button" onClick={() => { setEditing(p.id); setEditOffset(p.minute_offset); }} aria-label={`Edit offset for ${p.name}`} style={{ marginLeft: "var(--space-xs)", background: "none", border: "none", color: "var(--color-primary)", cursor: "pointer", textDecoration: "underline", fontFamily: "inherit", fontSize: "var(--text-label)" }}>edit</button>
                    </span>
                  )}
                </td>
                <td style={{ padding: "var(--space-sm)", textAlign: "center" }}>
                  <button
                    type="button"
                    onClick={() => setToggling({ id: p.id, action: p.status === "active" ? "disable" : "enable" })}
                    style={{ padding: "var(--space-xs) var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", background: "var(--color-surface)", font: "inherit", fontSize: "var(--text-label)", cursor: "pointer", color: p.status === "active" ? "var(--color-unavailable)" : "var(--color-fresh)" }}
                  >
                    {p.status === "active" ? "Disable" : "Enable"}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <ConfirmDialog
        open={!!toggling}
        title={toggling?.action === "disable" ? "Disable Provider" : "Enable Provider"}
        confirmText={toggling?.action === "disable" ? "DISABLE" : "ENABLE"}
        description={toggling?.action === "disable" ? "This will stop all scheduled collections for this provider. Existing data is retained." : "This will resume scheduled collections for this provider."}
        actionLabel={toggling?.action === "disable" ? "Disable" : "Enable"}
        onConfirm={() => { if (toggling) { onToggleStatus(toggling.id, toggling.action === "disable" ? "disabled" : "active"); setToggling(null); } }}
        onCancel={() => setToggling(null)}
      />
    </>
  );
}
