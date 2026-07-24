"use client";

import { useEffect } from "react";
import { ErrorPanel } from "@/components/ErrorPanel";

// Route-level error boundary (S-15 "500"). Shows the request_id for support
// correlation when present (ApiError carries it) and offers Retry (reset).
export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string; requestId?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // Surface for local debugging; never rendered to the user (no internals leak).
    console.error(error);
  }, [error]);

  return (
    <ErrorPanel
      title="Something went wrong"
      message="The server may be temporarily unavailable. Please try again."
      requestId={error.requestId}
      onRetry={reset}
    />
  );
}
