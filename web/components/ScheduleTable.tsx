"use client";

export interface ScheduleEntry {
  id: string;
  slot_type: string;
  provider_name: string | null;
  location_name: string | null;
  cron: string;
  status: string;
  last_run: string | null;
  next_run: string | null;
}

export interface ScheduleTableProps {
  schedules: ScheduleEntry[];
  onToggle: (id: string, action: "pause" | "resume") => void;
}

// S-13 Admin Schedules (doc 02 §4.13). Slot viewer with pause/resume (reversible,
// no ConfirmDialog needed). Shows slot_type, target, cron, status, timing.
export function ScheduleTable({ schedules, onToggle }: ScheduleTableProps) {
  return (
    <div className="tableWrap">
      <table style={{ width: "100%", borderCollapse: "collapse", minWidth: 650 }}>
        <thead>
          <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Type</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Target</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Cron</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Status</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Last Run</th>
            <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Next Run</th>
            <th scope="col" style={{ padding: "var(--space-sm)" }}><span className="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {schedules.map((s) => (
            <tr key={s.id} style={{ borderBottom: "1px solid var(--color-border)", height: 44 }}>
              <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)", fontSize: "var(--text-body-sm)" }}>{s.slot_type}</td>
              <td style={{ padding: "var(--space-sm)" }}>{s.provider_name && s.location_name ? `${s.provider_name} / ${s.location_name}` : "all"}</td>
              <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)", fontSize: "var(--text-body-sm)" }}>{s.cron}</td>
              <td style={{ padding: "var(--space-sm)" }}>
                <span style={{ padding: "2px var(--space-sm)", borderRadius: "var(--radius-full)", fontSize: "var(--text-label)", fontWeight: 500, background: s.status === "active" ? "var(--color-ranked)" : "var(--color-delayed)", color: "#fff" }}>{s.status}</span>
              </td>
              <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)", fontSize: "var(--text-body-sm)" }}>{s.last_run ? new Date(s.last_run).toLocaleString() : "—"}</td>
              <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)", fontSize: "var(--text-body-sm)" }}>{s.next_run ? new Date(s.next_run).toLocaleString() : "—"}</td>
              <td style={{ padding: "var(--space-sm)", textAlign: "center" }}>
                <button type="button" onClick={() => onToggle(s.id, s.status === "active" ? "pause" : "resume")} style={{ padding: "var(--space-xs) var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", background: "var(--color-surface)", font: "inherit", fontSize: "var(--text-label)", cursor: "pointer" }}>
                  {s.status === "active" ? "Pause" : "Resume"}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
