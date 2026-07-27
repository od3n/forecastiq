"use client";

import { Suspense, useState, useCallback } from "react";
import { useSearchParams, useRouter, usePathname } from "next/navigation";
import { useApi } from "@/lib/api/hooks";
import { absoluteLocal } from "@/lib/format";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { resetOnboarding } from "@/components/OnboardingDialog";

interface UserProfile {
  id: string;
  email: string;
  role: string;
  status: string;
  default_location_id: string | null;
  preferences: Record<string, unknown>;
  created_at: string;
  last_login_at: string | null;
}

interface UserData {
  user: UserProfile;
}

const TABS = [
  { id: "profile", label: "Profile" },
  { id: "keys", label: "API Keys" },
  { id: "preferences", label: "Preferences" },
  { id: "danger", label: "Danger Zone" },
] as const;

function SettingsContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const tab = searchParams.get("tab") ?? "profile";

  const setTab = (t: string) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("tab", t);
    router.replace(`${pathname}?${params.toString()}`, { scroll: false });
  };

  const { data: envelope, error, isLoading } = useApi<UserData>("/me");

  if (isLoading) {
    return (
      <section aria-labelledby="settings-heading">
        <h1 id="settings-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Settings</h1>
        <SkeletonBlock variant="row" count={5} />
      </section>
    );
  }

  if (error) {
    if (error.status === 401) {
      router.push("/auth/signin?return=/settings");
      return null;
    }
    return (
      <section aria-labelledby="settings-heading">
        <h1 id="settings-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Settings</h1>
        <ErrorPanel message="Unable to load your profile." requestId={error.requestId} onRetry={() => window.location.reload()} />
      </section>
    );
  }

  const user = envelope?.data?.user;

  return (
    <section aria-labelledby="settings-heading">
      <h1 id="settings-heading" style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Settings</h1>

      {/* Tabs (a11y: role=tablist + tab + tabpanel; ArrowLeft/Right) */}
      <div role="tablist" aria-label="Settings sections" style={{ display: "flex", gap: "var(--space-xs)", borderBottom: "1px solid var(--color-border)", marginBottom: "var(--space-lg)" }}>
        {TABS.map((t) => (
          <button
            key={t.id}
            role="tab"
            aria-selected={tab === t.id}
            aria-controls={`panel-${t.id}`}
            id={`tab-${t.id}`}
            onClick={() => setTab(t.id)}
            style={{
              padding: "var(--space-sm) var(--space-md)",
              border: "none",
              background: "transparent",
              font: "inherit",
              fontWeight: tab === t.id ? 600 : 400,
              color: tab === t.id ? "var(--color-primary)" : "var(--color-text-secondary)",
              borderBottom: tab === t.id ? "2px solid var(--color-primary)" : "2px solid transparent",
              cursor: "pointer",
              minHeight: 44,
            }}
          >
            {t.label}
          </button>
        ))}
      </div>

      {/* Tab panels */}
      {tab === "profile" && user && <ProfilePanel user={user} />}
      {tab === "keys" && <KeysPanel />}
      {tab === "preferences" && user && <PreferencesPanel user={user} />}
      {tab === "danger" && <DangerPanel />}
    </section>
  );
}

function ProfilePanel({ user }: { user: UserProfile }) {
  return (
    <div id="panel-profile" role="tabpanel" aria-labelledby="tab-profile">
      <h2 style={{ fontSize: "var(--text-h2)", fontWeight: 600, marginBottom: "var(--space-md)" }}>Profile</h2>
      <dl style={{ display: "grid", gridTemplateColumns: "1fr 2fr", gap: "var(--space-sm)", maxWidth: 400 }}>
        <dt style={{ color: "var(--color-text-secondary)" }}>Email</dt>
        <dd style={{ margin: 0 }}>{user.email}</dd>
        <dt style={{ color: "var(--color-text-secondary)" }}>Role</dt>
        <dd style={{ margin: 0, textTransform: "capitalize" }}>{user.role}</dd>
        <dt style={{ color: "var(--color-text-secondary)" }}>Status</dt>
        <dd style={{ margin: 0, textTransform: "capitalize" }}>{user.status}</dd>
        <dt style={{ color: "var(--color-text-secondary)" }}>Last login</dt>
        <dd style={{ margin: 0, fontFamily: "var(--font-data)" }}>{user.last_login_at ? absoluteLocal(user.last_login_at) : "—"}</dd>
      </dl>
    </div>
  );
}

function PreferencesPanel({ user }: { user: UserProfile }) {
  const [showOnboarding, setShowOnboarding] = useState(false);

  const handleResetOnboarding = useCallback(() => {
    resetOnboarding();
    setShowOnboarding(true);
    setTimeout(() => setShowOnboarding(false), 2000);
  }, []);

  const tzPref = (user.preferences?.tz_display as string) ?? "location";

  return (
    <div id="panel-preferences" role="tabpanel" aria-labelledby="tab-preferences">
      <h2 style={{ fontSize: "var(--text-h2)", fontWeight: 600, marginBottom: "var(--space-md)" }}>Preferences</h2>
      <div style={{ display: "flex", flexDirection: "column", gap: "var(--space-md)", maxWidth: 400 }}>
        <label style={{ display: "flex", flexDirection: "column", gap: "var(--space-xs)" }}>
          <span style={{ fontSize: "var(--text-label)", color: "var(--color-text-secondary)" }}>Timezone display</span>
          <select defaultValue={tzPref} style={{ padding: "var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", font: "inherit" }}>
            <option value="location">Location timezone</option>
            <option value="browser">Browser timezone</option>
          </select>
        </label>
        <div>
          <button type="button" onClick={handleResetOnboarding} style={{ padding: "var(--space-sm) var(--space-md)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", background: "var(--color-surface)", font: "inherit", cursor: "pointer" }}>
            Re-show onboarding
          </button>
          {showOnboarding && <span style={{ marginLeft: "var(--space-sm)", color: "var(--color-fresh)" }}>Onboarding will show on next visit.</span>}
        </div>
      </div>
    </div>
  );
}

// API Keys tab: list + create + revoke.
function KeysPanel() {
  const { data: keysEnv, mutate } = useApi<{ api_keys: ApiKeyEntry[] }>("/api-keys");
  const [creating, setCreating] = useState(false);
  const [newKey, setNewKey] = useState<string | null>(null);
  const [keyName, setKeyName] = useState("");
  const [copied, setCopied] = useState(false);
  const [revoking, setRevoking] = useState<string | null>(null);

  const keys = keysEnv?.data?.api_keys ?? [];

  const handleCreate = async () => {
    if (!keyName.trim()) return;
    setCreating(true);
    try {
      const resp = await fetch("/api/v1/api-keys", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: keyName }),
      });
      if (!resp.ok) throw new Error("Create failed");
      const body = await resp.json();
      setNewKey(body.data?.key ?? null);
      setKeyName("");
      mutate();
    } catch { /* ignore */ }
    setCreating(false);
  };

  const handleRevoke = async (id: string) => {
    await fetch(`/api/v1/api-keys/${id}`, { method: "DELETE" });
    setRevoking(null);
    mutate();
  };

  return (
    <div id="panel-keys" role="tabpanel" aria-labelledby="tab-keys">
      <h2 style={{ fontSize: "var(--text-h2)", fontWeight: 600, marginBottom: "var(--space-md)" }}>API Keys</h2>

      {/* Create key */}
      <div style={{ display: "flex", gap: "var(--space-sm)", marginBottom: "var(--space-md)", maxWidth: 400 }}>
        <input
          type="text"
          placeholder="Key name"
          value={keyName}
          onChange={(e) => setKeyName(e.target.value)}
          style={{ flex: 1, padding: "var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", font: "inherit" }}
        />
        <button type="button" onClick={handleCreate} disabled={creating || !keyName.trim()} style={{ padding: "var(--space-sm) var(--space-md)", background: "var(--color-primary)", color: "#fff", border: "none", borderRadius: "var(--radius-md)", font: "inherit", fontWeight: 500, cursor: "pointer", opacity: creating || !keyName.trim() ? 0.5 : 1 }}>
          Create
        </button>
      </div>

      {/* Show newly created key once */}
      {newKey && (
        <div role="alert" style={{ background: "var(--color-surface-secondary)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", padding: "var(--space-md)", marginBottom: "var(--space-md)", maxWidth: 500 }}>
          <p style={{ fontWeight: 500, marginBottom: "var(--space-xs)" }}>Key created — copy it now (shown once):</p>
          <code style={{ fontFamily: "var(--font-data)", wordBreak: "break-all" }}>{newKey}</code>
          <button type="button" onClick={() => { navigator.clipboard.writeText(newKey); setCopied(true); setTimeout(() => setCopied(false), 2000); }} style={{ marginLeft: "var(--space-sm)", padding: "var(--space-xs) var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", background: "var(--color-surface)", font: "inherit", cursor: "pointer" }}>
            {copied ? "Copied" : "Copy"}
          </button>
          <span aria-live="polite" style={{ position: "absolute", width: 1, height: 1, overflow: "hidden", clip: "rect(0,0,0,0)" }}>{copied ? "Copied to clipboard" : ""}</span>
        </div>
      )}

      {/* Key list */}
      {keys.length === 0 ? (
        <p style={{ color: "var(--color-text-secondary)" }}>No API keys yet.</p>
      ) : (
        <table style={{ width: "100%", borderCollapse: "collapse", maxWidth: 600 }}>
          <thead>
            <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Name</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Prefix</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Created</th>
              <th scope="col" style={{ padding: "var(--space-sm)" }}></th>
            </tr>
          </thead>
          <tbody>
            {keys.map((k) => (
              <tr key={k.id} style={{ borderBottom: "1px solid var(--color-border)" }}>
                <td style={{ padding: "var(--space-sm)" }}>{k.name}</td>
                <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)" }}>{k.key_prefix}</td>
                <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)", fontSize: "var(--text-body-sm)" }}>{new Date(k.created_at).toLocaleDateString()}</td>
                <td style={{ padding: "var(--space-sm)" }}>
                  {!k.revoked_at && (
                    <button type="button" onClick={() => setRevoking(k.id)} style={{ color: "var(--color-unavailable)", background: "none", border: "none", font: "inherit", cursor: "pointer", textDecoration: "underline" }}>Revoke</button>
                  )}
                  {k.revoked_at && <span style={{ color: "var(--color-text-muted)" }}>Revoked</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {/* Revoke confirm */}
      <ConfirmDialog
        open={!!revoking}
        title="Revoke API Key"
        confirmText="REVOKE"
        description="This key will immediately stop working. This cannot be undone."
        actionLabel="Revoke Key"
        onConfirm={() => revoking && handleRevoke(revoking)}
        onCancel={() => setRevoking(null)}
      />
    </div>
  );
}

interface ApiKeyEntry {
  id: string;
  name: string;
  key_prefix: string;
  scopes: string[];
  created_at: string;
  revoked_at: string | null;
  last_used_at: string | null;
}

// Danger Zone tab: export + delete account.
function DangerPanel() {
  const [showDelete, setShowDelete] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [exportMsg, setExportMsg] = useState<string | null>(null);
  const router = useRouter();

  const handleExport = async () => {
    setExporting(true);
    try {
      const resp = await fetch("/api/v1/me/export", { method: "POST" });
      if (resp.ok) setExportMsg("Export started. Check back shortly for the download link.");
      else setExportMsg("Export request failed. Please try again.");
    } catch { setExportMsg("Network error."); }
    setExporting(false);
  };

  const handleDelete = async () => {
    await fetch("/api/v1/me", { method: "DELETE" });
    setShowDelete(false);
    router.push("/auth/signin");
  };

  return (
    <div id="panel-danger" role="tabpanel" aria-labelledby="tab-danger">
      <h2 style={{ fontSize: "var(--text-h2)", fontWeight: 600, marginBottom: "var(--space-md)", color: "var(--color-unavailable)" }}>Danger Zone</h2>
      <div style={{ display: "flex", flexDirection: "column", gap: "var(--space-md)", maxWidth: 400 }}>
        {/* Export */}
        <div>
          <p style={{ marginBottom: "var(--space-sm)", color: "var(--color-text-secondary)" }}>Export all your account data (profile, keys, audit trail) as a JSON file.</p>
          <button type="button" onClick={handleExport} disabled={exporting} style={{ padding: "var(--space-sm) var(--space-md)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-md)", background: "var(--color-surface)", font: "inherit", cursor: "pointer" }}>
            {exporting ? "Exporting..." : "Export my data"}
          </button>
          {exportMsg && <p style={{ marginTop: "var(--space-xs)", color: "var(--color-text-secondary)", fontSize: "var(--text-body-sm)" }}>{exportMsg}</p>}
        </div>
        {/* Delete */}
        <div>
          <p style={{ marginBottom: "var(--space-sm)", color: "var(--color-text-secondary)" }}>Permanently delete your account and all associated data. This cannot be undone.</p>
          <button type="button" onClick={() => setShowDelete(true)} style={{ padding: "var(--space-sm) var(--space-md)", background: "var(--color-unavailable)", color: "#fff", border: "none", borderRadius: "var(--radius-md)", font: "inherit", fontWeight: 500, cursor: "pointer" }}>
            Delete my account
          </button>
        </div>
      </div>
      <ConfirmDialog
        open={showDelete}
        title="Delete Account"
        confirmText="DELETE"
        description="This will permanently remove your account, API keys, and personal data. Audit records are anonymized. This action is irreversible."
        actionLabel="Delete Forever"
        onConfirm={handleDelete}
        onCancel={() => setShowDelete(false)}
      />
    </div>
  );
}

export default function SettingsPage() {
  return (
    <Suspense fallback={<SkeletonBlock variant="row" count={5} />}>
      <SettingsContent />
    </Suspense>
  );
}
