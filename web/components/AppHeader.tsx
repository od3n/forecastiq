"use client";

import Link from "next/link";
import { Suspense, useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";
import { useGlobalParams } from "@/lib/state/use-global-params";
import { useApi } from "@/lib/api/hooks";
import { signOut, useSession } from "@/lib/auth/session";
import { LocationSelector } from "./LocationSelector";
import { HorizonSelector } from "./HorizonSelector";
import styles from "./AppHeader.module.css";

interface MeData {
  user: { role: string };
}

// Session-aware account area: Sign in link when signed out; when signed in, a
// disclosure menu under the email (opens on hover, click, or keyboard; closes
// on Escape, blur, or pointer-out) holding Settings, Admin (role-gated) and
// Sign out. Sign-out is a full navigation so all SWR caches restart
// unauthenticated.
function AccountArea({ isAdmin }: { isAdmin: boolean }) {
  const session = useSession();
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  if (session.status === "loading") return <span className={styles.account} />;

  if (session.status === "signed-out") {
    return (
      <Link href="/auth/signin" className={styles.account}>
        Sign in
      </Link>
    );
  }

  return (
    <div
      ref={wrapRef}
      className={styles.menuWrap}
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
      onKeyDown={(e) => {
        if (e.key === "Escape") setOpen(false);
      }}
      onBlur={(e) => {
        if (!wrapRef.current?.contains(e.relatedTarget as Node | null)) setOpen(false);
      }}
    >
      <button
        type="button"
        className={styles.menuTrigger}
        aria-expanded={open}
        aria-haspopup="true"
        onClick={() => setOpen((v) => !v)}
      >
        <span className={styles.accountEmail}>{session.email ?? "Account"}</span>
        <span aria-hidden="true" className={styles.caret}>
          ▾
        </span>
      </button>
      {open && (
        <div className={styles.menu}>
          <Link href="/settings" className={styles.menuItem} onClick={() => setOpen(false)}>
            Settings
          </Link>
          {isAdmin && (
            <Link href="/admin/health" className={styles.menuItem} onClick={() => setOpen(false)}>
              Admin
            </Link>
          )}
          <button
            type="button"
            className={styles.menuItem}
            onClick={() => {
              void signOut().then(() => window.location.assign("/"));
            }}
          >
            Sign out
          </button>
        </div>
      )}
    </div>
  );
}

// Inner header content (uses hooks; needs Suspense boundary for useSearchParams).
function HeaderInner() {
  const { locationId, horizonMinutes, setParams } = useGlobalParams();
  const pathname = usePathname();
  const session = useSession();

  // Role-gated Admin entry (doc 02 §3.1): /me is only fetched with a session,
  // and the account menu shows Admin only for role=admin (the admin layout
  // re-guards).
  const signedIn = session.status === "signed-in";
  const { data: me } = useApi<MeData>(signedIn ? "/me" : null);
  const isAdmin = me?.data?.user?.role === "admin";

  // The FvA daily comparison always shows the full 24-hour day, so only +24h
  // is selectable there; any other selection is clamped to 1440.
  const isDailyComparison = pathname.startsWith("/forecast-comparison");
  const allowedMinutes = isDailyComparison ? [1440] : undefined;
  useEffect(() => {
    if (isDailyComparison && horizonMinutes !== 1440) {
      setParams({ horizon_minutes: "1440" });
    }
  }, [isDailyComparison, horizonMinutes, setParams]);

  return (
    <div className={styles.inner}>
      <Link href="/" className={styles.logo}>
        ForecastIQ
      </Link>
      <nav className={styles.nav} aria-label="Primary">
        <Link href="/" className={styles.navLink}>
          Overview
        </Link>
        <Link href="/trends" className={styles.navLink}>
          Trends
        </Link>
        <Link href="/forecast-comparison" className={styles.navLink}>
          Compare
        </Link>
        <Link href="/methodology" className={styles.navLink}>
          Methodology
        </Link>
      </nav>
      <span className={styles.spacer} />
      <LocationSelector
        selected={locationId}
        onChange={(id) => setParams({ location_id: id })}
      />
      <HorizonSelector
        selected={horizonMinutes}
        onChange={(m) => setParams({ horizon_minutes: String(m) })}
        allowedMinutes={allowedMinutes}
      />
      <AccountArea isAdmin={isAdmin} />
    </div>
  );
}

// Global header shell (doc 02 §3.1): public nav + global selectors + the
// session-aware account menu (Settings / role-gated Admin / Sign out).
export function AppHeader() {
  return (
    <header className={styles.header}>
      <Suspense fallback={<div className={styles.inner} />}>
        <HeaderInner />
      </Suspense>
    </header>
  );
}
