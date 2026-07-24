import { expect, afterEach } from "vitest";
import { cleanup } from "@testing-library/react";
import { toHaveNoViolations } from "jest-axe";

// Register the axe matcher and unmount rendered trees between tests.
expect.extend(toHaveNoViolations);
afterEach(() => cleanup());
