"use client";

import Link from "next/link";
import { Suspense, useEffect } from "react";
import { usePathname } from "next/navigation";
import { useGlobalParams } from "@/lib/state/use-global-params";
import { LocationSelector } from "./LocationSelector";
import { HorizonSelector } from "./HorizonSelector";
import styles from "./AppHeader.module.css";

// Inner header content (uses hooks; needs Suspense boundary for useSearchParams).
function HeaderInner() {
  const { locationId, horizonMinutes, setParams } = useGlobalParams();
  const pathname = usePathname();

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
        <Link href="/settings" className={styles.navLink}>
          Settings
        </Link>
        <Link href="/admin/health" className={styles.navLink}>
          Admin
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
      <Link href="/auth/signin" className={styles.account}>
        Sign in
      </Link>
    </div>
  );
}

// Global header shell (doc 02 §3.1). Role-gated items (Admin, Settings) are
// wired when the session-aware header lands; the foundation ships the public nav
// + global selectors + account entry point.
export function AppHeader() {
  return (
    <header className={styles.header}>
      <Suspense fallback={<div className={styles.inner} />}>
        <HeaderInner />
      </Suspense>
    </header>
  );
}
