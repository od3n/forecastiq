"use client";

import { useState } from "react";
import { ConfirmDialog } from "./ConfirmDialog";
import { absoluteLocal } from "@/lib/format";

export interface UserEntry {
  id: string;
  email: string;
  role: string;
  status: string;
  created_at: string;
  last_login_at: string | null;
}

export interface UserAdminTableProps {
  users: UserEntry[];
  onRoleChange: (id: string, newRole: string) => void;
  onToggleStatus: (id: string, newStatus: string) => void;
  onExport: (id: string) => void;
}

// S-14 Admin Users table (doc 02 §4.14). Role management (user/admin dropdown +
// ConfirmDialog), disable/enable (ConfirmDialog), admin-triggered export button.
export function UserAdminTable({ users, onRoleChange, onToggleStatus, onExport }: UserAdminTableProps) {
  const [confirm, setConfirm] = useState<{ id: string; action: string; text: string; desc: string; label: string } | null>(null);
  const [exportedId, setExportedId] = useState<string | null>(null);

  const handleExport = (id: string) => {
    onExport(id);
    setExportedId(id);
    setTimeout(() => setExportedId(null), 3000);
  };

  return (
    <>
      <div className="tableWrap">
        <table style={{ width: "100%", borderCollapse: "collapse", minWidth: 700 }}>
          <thead>
            <tr style={{ background: "var(--color-surface-secondary)", fontSize: "var(--text-label)", textTransform: "uppercase" }}>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Email</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Role</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Status</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Created</th>
              <th scope="col" style={{ padding: "var(--space-sm)", textAlign: "left" }}>Last Login</th>
              <th scope="col" style={{ padding: "var(--space-sm)" }}><span className="sr-only">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id} style={{ borderBottom: "1px solid var(--color-border)", height: 48 }}>
                <td style={{ padding: "var(--space-sm)" }}>{u.email}</td>
                <td style={{ padding: "var(--space-sm)" }}>
                  <select
                    value={u.role}
                    aria-label={`Role for ${u.email}`}
                    onChange={(e) => {
                      const newRole = e.target.value;
                      setConfirm({ id: u.id, action: `role-${newRole}`, text: newRole.toUpperCase(), desc: `Change role for ${u.email} to ${newRole}. This takes effect immediately.`, label: "Change Role" });
                    }}
                    style={{ padding: "var(--space-xs) var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", font: "inherit", fontSize: "var(--text-body-sm)" }}
                  >
                    <option value="user">user</option>
                    <option value="admin">admin</option>
                  </select>
                </td>
                <td style={{ padding: "var(--space-sm)" }}>
                  <span style={{ padding: "2px var(--space-sm)", borderRadius: "var(--radius-full)", fontSize: "var(--text-label)", fontWeight: 500, background: u.status === "active" ? "var(--color-ranked)" : "var(--color-border)", color: u.status === "active" ? "#fff" : "var(--color-text-secondary)" }}>{u.status}</span>
                </td>
                <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)", fontSize: "var(--text-body-sm)" }}>{absoluteLocal(u.created_at)}</td>
                <td style={{ padding: "var(--space-sm)", fontFamily: "var(--font-data)", fontSize: "var(--text-body-sm)" }}>{u.last_login_at ? absoluteLocal(u.last_login_at) : "—"}</td>
                <td style={{ padding: "var(--space-sm)", display: "flex", gap: "var(--space-xs)", alignItems: "center" }}>
                  <button type="button" onClick={() => setConfirm({ id: u.id, action: u.status === "active" ? "disable" : "enable", text: u.status === "active" ? "DISABLE" : "ENABLE", desc: u.status === "active" ? `Disable ${u.email}. They will not be able to log in.` : `Re-enable ${u.email}.`, label: u.status === "active" ? "Disable" : "Enable" })} style={{ padding: "var(--space-xs) var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", background: "var(--color-surface)", font: "inherit", fontSize: "var(--text-label)", cursor: "pointer" }}>
                    {u.status === "active" ? "Disable" : "Enable"}
                  </button>
                  <button type="button" onClick={() => handleExport(u.id)} style={{ padding: "var(--space-xs) var(--space-sm)", border: "1px solid var(--color-border)", borderRadius: "var(--radius-sm)", background: "var(--color-surface)", font: "inherit", fontSize: "var(--text-label)", cursor: "pointer" }}>
                    Export
                  </button>
                  {exportedId === u.id && <span aria-live="polite" style={{ fontSize: "var(--text-label)", color: "var(--color-fresh)" }}>Queued</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <ConfirmDialog
        open={!!confirm}
        title={confirm?.label ?? ""}
        confirmText={confirm?.text ?? ""}
        description={confirm?.desc ?? ""}
        actionLabel={confirm?.label ?? "Confirm"}
        onConfirm={() => {
          if (!confirm) return;
          if (confirm.action.startsWith("role-")) onRoleChange(confirm.id, confirm.action.replace("role-", ""));
          else onToggleStatus(confirm.id, confirm.action === "disable" ? "disabled" : "active");
          setConfirm(null);
        }}
        onCancel={() => setConfirm(null)}
      />
    </>
  );
}
