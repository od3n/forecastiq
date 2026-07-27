"use client";

import { useMemo, useState } from "react";
import { ConfirmDialog } from "./ConfirmDialog";

/** ISO 3166-1 alpha-2 codes with display names, derived from Intl (no static
 * country list dependency): every AA–ZZ pair whose DisplayNames resolution
 * differs from the code itself is a real region. */
function buildCountryOptions(): { code: string; name: string }[] {
  try {
    const display = new Intl.DisplayNames(["en"], { type: "region" });
    const out: { code: string; name: string }[] = [];
    for (let a = 65; a <= 90; a++) {
      for (let b = 65; b <= 90; b++) {
        const code = String.fromCharCode(a) + String.fromCharCode(b);
        const name = display.of(code);
        if (name && name !== code) out.push({ code, name });
      }
    }
    return out.sort((x, y) => x.name.localeCompare(y.name));
  } catch {
    return [];
  }
}

function buildTimezoneOptions(): string[] {
  try {
    return Intl.supportedValuesOf("timeZone");
  } catch {
    return [];
  }
}

export interface LocationEntry {
  id: string;
  name: string;
  country_code: string;
  latitude: number;
  longitude: number;
  timezone: string;
  status: string;
}

export interface CreateLocationData {
  name: string;
  country_code: string;
  latitude: number;
  longitude: number;
  timezone: string;
  allow_near_duplicate?: boolean;
  override_reason?: string;
}

export interface CreateResult {
  error: string;
  /** True when the failure is a BR-LOC-01 proximity conflict (override possible). */
  conflict?: boolean;
}

export interface LocationAdminTableProps {
  locations: LocationEntry[];
  onToggleStatus: (id: string, newStatus: string) => void;
  onCreate: (data: CreateLocationData) => Promise<CreateResult | null>;
}

// S-12 Admin Locations table (doc 02 §4.12). CRUD with status lifecycle
// (active/deactivated). Proximity warning (409) shown inline on create.
export function LocationAdminTable({ locations, onToggleStatus, onCreate }: LocationAdminTableProps) {
  const [toggling, setToggling] = useState<{ id: string; action: string } | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState({ name: "", country_code: "", latitude: "", longitude: "", timezone: "" });
  const [createError, setCreateError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  // BR-LOC-01 override: revealed after a proximity conflict; requires a reason.
  const [showOverride, setShowOverride] = useState(false);
  const [allowNearDuplicate, setAllowNearDuplicate] = useState(false);
  const [overrideReason, setOverrideReason] = useState("");

  const countryOptions = useMemo(buildCountryOptions, []);
  const timezoneOptions = useMemo(buildTimezoneOptions, []);

  const resetForm = () => {
    setForm({ name: "", country_code: "", latitude: "", longitude: "", timezone: "" });
    setShowOverride(false);
    setAllowNearDuplicate(false);
    setOverrideReason("");
    setCreateError(null);
  };

  const handleCreate = async () => {
    setCreating(true);
    setCreateError(null);
    const result = await onCreate({
      name: form.name,
      country_code: form.country_code,
      latitude: Number(form.latitude),
      longitude: Number(form.longitude),
      timezone: form.timezone,
      ...(allowNearDuplicate ? { allow_near_duplicate: true, override_reason: overrideReason } : {}),
    });
    if (result) {
      setCreateError(result.error);
      if (result.conflict) setShowOverride(true);
    } else {
      setShowCreate(false);
      resetForm();
    }
    setCreating(false);
  };

  return (
    <>
      <button type="button" onClick={() => setShowCreate((s) => !s)} style={{ marginBottom: "var(--space-md)", padding: "var(--space-sm) var(--space-md)", background: "var(--color-primary)", color: "#fff", border: "none", borderRadius: "var(--radius-md)", font: "inherit", fontWeight: 500, cursor: "pointer" }}>
        {showCreate ? "Cancel" : "Add Location"}
      </button>

      {showCreate && (
        <div style={{ marginBottom: "var(--space-md)", padding: "var(--space-md)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", maxWidth: 500, display: "flex", flexDirection: "column", gap: "var(--space-sm)" }}>
          <input placeholder="Name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} style={{ padding: "var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", font: "inherit" }} />
          <div style={{ display: "flex", gap: "var(--space-sm)" }}>
            <input placeholder="Lat" type="number" step="any" value={form.latitude} onChange={(e) => setForm({ ...form, latitude: e.target.value })} style={{ flex: 1, padding: "var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", font: "inherit" }} />
            <input placeholder="Lon" type="number" step="any" value={form.longitude} onChange={(e) => setForm({ ...form, longitude: e.target.value })} style={{ flex: 1, padding: "var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", font: "inherit" }} />
          </div>
          <div style={{ display: "flex", gap: "var(--space-sm)" }}>
            <input
              placeholder="Country (e.g. MY)"
              list="country-codes"
              autoComplete="off"
              value={form.country_code}
              onChange={(e) => setForm({ ...form, country_code: e.target.value.toUpperCase() })}
              style={{ flex: 1, padding: "var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", font: "inherit" }}
            />
            <datalist id="country-codes">
              {countryOptions.map((c) => (
                <option key={c.code} value={c.code}>{c.name}</option>
              ))}
            </datalist>
            <input
              placeholder="Timezone (e.g. Asia/Kuala_Lumpur)"
              list="iana-timezones"
              autoComplete="off"
              value={form.timezone}
              onChange={(e) => setForm({ ...form, timezone: e.target.value })}
              style={{ flex: 1, padding: "var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", font: "inherit" }}
            />
            <datalist id="iana-timezones">
              {timezoneOptions.map((tz) => (
                <option key={tz} value={tz} />
              ))}
            </datalist>
          </div>
          {createError && <p role="alert" style={{ color: "var(--color-unavailable)", fontSize: "var(--text-body-sm)" }}>{createError}</p>}
          {showOverride && (
            <div style={{ display: "flex", flexDirection: "column", gap: "var(--space-xs)", padding: "var(--space-sm)", background: "var(--color-surface-secondary)", borderRadius: "var(--radius-sm)" }}>
              <label style={{ display: "flex", alignItems: "center", gap: "var(--space-xs)", fontSize: "var(--text-body-sm)" }}>
                <input type="checkbox" checked={allowNearDuplicate} onChange={(e) => setAllowNearDuplicate(e.target.checked)} />
                Create anyway (near-duplicate override — audited)
              </label>
              {allowNearDuplicate && (
                <input
                  placeholder="Reason for override (required)"
                  value={overrideReason}
                  onChange={(e) => setOverrideReason(e.target.value)}
                  style={{ padding: "var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", font: "inherit" }}
                />
              )}
            </div>
          )}
          <button type="button" onClick={handleCreate} disabled={creating || !form.name || (allowNearDuplicate && !overrideReason.trim())} style={{ alignSelf: "flex-start", padding: "var(--space-sm) var(--space-md)", background: "var(--color-primary)", color: "#fff", border: "none", borderRadius: "var(--radius-md)", font: "inherit", fontWeight: 500, cursor: "pointer", opacity: creating || !form.name || (allowNearDuplicate && !overrideReason.trim()) ? 0.5 : 1 }}>
            {creating ? "Creating..." : allowNearDuplicate ? "Create with override" : "Create"}
          </button>
        </div>
      )}

      <div className="tableWrap">
        <table style={{ width: "100%", borderCollapse: "collapse", minWidth: 600 }}>
          <thead>
            <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Name</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Country</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>Lat</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "right" }}>Lon</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Timezone</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Status</th>
              <th scope="col" style={{ padding: "var(--space-sm)" }}><span className="sr-only">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            {locations.map((loc) => (
              <tr key={loc.id} style={{ borderBottom: "1px solid var(--color-border)", height: 44 }}>
                <td style={{ padding: "var(--space-sm)", fontWeight: 500 }}>{loc.name}</td>
                <td style={{ padding: "var(--space-sm)" }}>{loc.country_code}</td>
                <td style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)" }}>{loc.latitude.toFixed(4)}</td>
                <td style={{ padding: "var(--space-sm)", textAlign: "right", fontFamily: "var(--font-data)" }}>{loc.longitude.toFixed(4)}</td>
                <td style={{ padding: "var(--space-sm)", fontSize: "var(--text-body-sm)" }}>{loc.timezone}</td>
                <td style={{ padding: "var(--space-sm)" }}>
                  <span style={{ padding: "2px var(--space-sm)", borderRadius: "var(--radius-full)", fontSize: "var(--text-label)", fontWeight: 500, background: loc.status === "active" ? "var(--color-ranked)" : "var(--color-border)", color: loc.status === "active" ? "#fff" : "var(--color-text-secondary)" }}>{loc.status}</span>
                </td>
                <td style={{ padding: "var(--space-sm)", textAlign: "center" }}>
                  <button type="button" onClick={() => setToggling({ id: loc.id, action: loc.status === "active" ? "deactivate" : "activate" })} style={{ padding: "var(--space-xs) var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", background: "var(--color-surface)", font: "inherit", fontSize: "var(--text-label)", cursor: "pointer" }}>
                    {loc.status === "active" ? "Deactivate" : "Activate"}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <ConfirmDialog
        open={!!toggling}
        title={toggling?.action === "deactivate" ? "Deactivate Location" : "Activate Location"}
        confirmText={toggling?.action === "deactivate" ? "DEACTIVATE" : "ACTIVATE"}
        description={toggling?.action === "deactivate" ? "Collections for this location will stop. Existing data is retained." : "Collections will resume for this location."}
        actionLabel={toggling?.action === "deactivate" ? "Deactivate" : "Activate"}
        onConfirm={() => { if (toggling) { onToggleStatus(toggling.id, toggling.action === "deactivate" ? "deactivated" : "active"); setToggling(null); } }}
        onCancel={() => setToggling(null)}
      />
    </>
  );
}
