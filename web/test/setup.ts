import { expect, afterEach } from "vitest";
import { cleanup } from "@testing-library/react";
import { toHaveNoViolations } from "jest-axe";

// Register the axe matcher and unmount rendered trees between tests.
expect.extend(toHaveNoViolations);
afterEach(() => cleanup());

// Recharts needs ResizeObserver (not in jsdom). Provide a no-op mock.
class ResizeObserverMock {
  observe() {}
  unobserve() {}
  disconnect() {}
}
global.ResizeObserver = ResizeObserverMock as unknown as typeof ResizeObserver;
