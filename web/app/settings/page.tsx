"use client";

import { Suspense, useState, useCallback } from "react";
import { useSearchParams, useRouter, usePathname } from "next/navigation";
import { useApi } from "@/lib/api/hooks";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";
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
        <dd style={{ margin: 0, fontFamily: "var(--font-data)" }}>{user.last_login_at ? new Date(user.last_login_at).toLocaleString() : "—"}</dd>
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

// Placeholder panels for sub-commit 2
function KeysPanel() {
  return (
    <div id="panel-keys" role="tabpanel" aria-labelledby="tab-keys">
      <h2 style={{ fontSize: "var(--text-h2)", fontWeight: 600, marginBottom: "var(--space-md)" }}>API Keys</h2>
      <p style={{ color: "var(--color-text-secondary)" }}>API key management loads in the next sub-commit.</p>
    </div>
  );
}

function DangerPanel() {
  return (
    <div id="panel-danger" role="tabpanel" aria-labelledby="tab-danger">
      <h2 style={{ fontSize: "var(--text-h2)", fontWeight: 600, marginBottom: "var(--space-md)", color: "var(--color-unavailable)" }}>Danger Zone</h2>
      <p style={{ color: "var(--color-text-secondary)" }}>Account export and deletion load in the next sub-commit.</p>
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
