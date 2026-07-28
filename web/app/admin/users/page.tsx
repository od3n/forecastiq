"use client";

import { useCallback } from "react";
import { useApi } from "@/lib/api/hooks";
import { apiBase } from "@/lib/api/client";
import { authHeaders } from "@/lib/auth/session";
import { UserAdminTable, type UserEntry } from "@/components/UserAdminTable";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";

interface UsersData { users: UserEntry[]; }

export default function AdminUsersPage() {
  const { data: envelope, error, isLoading, mutate } = useApi<UsersData>("/admin/users");

  const handleRoleChange = useCallback(async (id: string, newRole: string) => {
    await fetch(`${apiBase}/admin/users/${id}/role`, { method: "PATCH", headers: await authHeaders(), body: JSON.stringify({ role: newRole }) });
    mutate();
  }, [mutate]);

  const handleToggleStatus = useCallback(async (id: string, newStatus: string) => {
    await fetch(`${apiBase}/admin/users/${id}/status`, { method: "PATCH", headers: await authHeaders(), body: JSON.stringify({ status: newStatus }) });
    mutate();
  }, [mutate]);

  const handleExport = useCallback(async (id: string) => {
    await fetch(`${apiBase}/admin/users/${id}/export`, { method: "POST", headers: await authHeaders() });
  }, []);

  if (isLoading) return (<section><h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Users</h1><SkeletonBlock variant="row" count={4} /></section>);
  if (error) return (<section><h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Users</h1><ErrorPanel message="Unable to load users." requestId={error.requestId} onRetry={() => mutate()} /></section>);

  const users = envelope?.data?.users ?? [];

  return (
    <section>
      <h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Users</h1>
      {users.length === 0 ? <p style={{ color: "var(--color-text-secondary)" }}>No users.</p> : <UserAdminTable users={users} onRoleChange={handleRoleChange} onToggleStatus={handleToggleStatus} onExport={handleExport} />}
    </section>
  );
}
