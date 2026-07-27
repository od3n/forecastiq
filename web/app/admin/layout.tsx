"use client";

import { type ReactNode } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useApi } from "@/lib/api/hooks";
import { ErrorPanel } from "@/components/ErrorPanel";
import { SkeletonBlock } from "@/components/SkeletonBlock";

interface MeData {
  user: { role: string };
}

const ADMIN_NAV = [
  { href: "/admin/dashboard", label: "Dashboard" },
  { href: "/admin/health", label: "Health" },
  { href: "/admin/providers", label: "Providers" },
  { href: "/admin/locations", label: "Locations" },
  { href: "/admin/schedules", label: "Schedules" },
  { href: "/admin/forecasts", label: "Raw Forecasts" },
  { href: "/admin/users", label: "Users" },
] as const;

// Admin layout: role guard (GET /me; 401→redirect, non-admin→403 ErrorPanel)
// + admin sub-nav (doc 02 §3.2). All admin screens nest under this layout.
export default function AdminLayout({ children }: { children: ReactNode }) {
  const { data: envelope, error, isLoading } = useApi<MeData>("/me");
  const pathname = usePathname();

  if (isLoading) return <SkeletonBlock variant="row" count={3} />;

  if (error) {
    if (error.status === 401) {
      if (typeof window !== "undefined") window.location.href = `/auth/signin?return=${pathname}`;
      return null;
    }
    return <ErrorPanel title="Administrator access required" message="This section is restricted to platform operators." />;
  }

  const role = envelope?.data?.user?.role;
  if (role !== "admin") {
    return <ErrorPanel title="Administrator access required" message="This section is restricted to platform operators. Sign in with a different account." />;
  }

  return (
    <div>
      <nav role="navigation" aria-label="Admin sections" style={{ display: "flex", gap: "var(--space-sm)", borderBottom: "1px solid var(--color-border)", marginBottom: "var(--space-lg)", paddingBottom: "var(--space-sm)" }}>
        {ADMIN_NAV.map((item) => (
          <Link
            key={item.href}
            href={item.href}
            style={{
              padding: "var(--space-xs) var(--space-sm)",
              borderRadius: "var(--radius-md)",
              fontSize: "var(--text-body-sm)",
              fontWeight: pathname === item.href ? 600 : 400,
              color: pathname === item.href ? "var(--color-primary)" : "var(--color-text-secondary)",
              textDecoration: "none",
              background: pathname === item.href ? "var(--color-surface-secondary)" : "transparent",
            }}
          >
            {item.label}
          </Link>
        ))}
      </nav>
      {children}
    </div>
  );
}
