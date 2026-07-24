import type { ReactElement } from "react";
import { render } from "@testing-library/react";
import { axe } from "jest-axe";
import { describe, it, expect } from "vitest";
import { FreshnessBadge } from "@/components/FreshnessBadge";
import { StaleBanner } from "@/components/StaleBanner";
import { PartialWarnings } from "@/components/PartialWarnings";
import { EmptyState } from "@/components/EmptyState";
import { ErrorPanel } from "@/components/ErrorPanel";
import type { FreshnessState } from "@/lib/api/types";

// Component-level axe checks (a11y §5 "axe-core in CI on all screens"). Rendering
// fragments in isolation, so the document-scoped rules (landmark/region/single-
// H1) are disabled — those belong to the full-page checks that arrive with the
// screens (WP-21). Color-contrast needs layout and is not evaluated in jsdom.
async function expectNoViolations(ui: ReactElement) {
  const { container } = render(ui);
  const results = await axe(container, {
    rules: {
      region: { enabled: false },
      "landmark-one-main": { enabled: false },
      "page-has-heading-one": { enabled: false },
    },
  });
  expect(results).toHaveNoViolations();
}

describe("a11y: envelope rendering primitives", () => {
  it("FreshnessBadge renders every state without violations", async () => {
    const states: FreshnessState[] = ["fresh", "delayed", "stale", "unavailable"];
    for (const state of states) {
      await expectNoViolations(
        <FreshnessBadge state={state} lastUpdated="2026-07-24T00:00:00Z" />,
      );
    }
  });

  it("StaleBanner", async () => {
    await expectNoViolations(<StaleBanner lastUpdated="2026-07-24T00:00:00Z" />);
  });

  it("PartialWarnings", async () => {
    await expectNoViolations(
      <PartialWarnings
        warnings={[{ code: "provider_unavailable", message: "OpenWeather temporarily unavailable" }]}
      />,
    );
  });

  it("EmptyState", async () => {
    await expectNoViolations(
      <EmptyState variant="no-data" title="Collecting data" description="First data within ~1 hour." />,
    );
  });

  it("ErrorPanel", async () => {
    await expectNoViolations(
      <ErrorPanel message="The server may be temporarily unavailable." requestId="abc-123" onRetry={() => {}} />,
    );
  });
});
