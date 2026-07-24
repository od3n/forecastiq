"use client";

import { useCallback } from "react";
import { useApi } from "@/lib/api/hooks";
import { ScheduleTable, type ScheduleEntry } from "@/components/ScheduleTable";
import { SkeletonBlock } from "@/components/SkeletonBlock";
import { ErrorPanel } from "@/components/ErrorPanel";

interface SchedulesData { schedules: ScheduleEntry[]; }

export default function AdminSchedulesPage() {
  const { data: envelope, error, isLoading, mutate } = useApi<SchedulesData>("/admin/schedules");

  const handleToggle = useCallback(async (id: string, action: "pause" | "resume") => {
    await fetch(`/api/v1/admin/schedules/${id}/${action}`, { method: "POST" });
    mutate();
  }, [mutate]);

  if (isLoading) return (<section><h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Schedules</h1><SkeletonBlock variant="row" count={5} /></section>);
  if (error) return (<section><h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Schedules</h1><ErrorPanel message="Unable to load schedules." requestId={error.requestId} onRetry={() => mutate()} /></section>);

  const schedules = envelope?.data?.schedules ?? [];

  return (
    <section>
      <h1 style={{ fontSize: "var(--text-display)", fontWeight: 700, marginBottom: "var(--space-md)" }}>Schedules</h1>
      {schedules.length === 0 ? <p style={{ color: "var(--color-text-secondary)" }}>No schedules configured.</p> : <ScheduleTable schedules={schedules} onToggle={handleToggle} />}
    </section>
  );
}
