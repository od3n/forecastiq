import "vitest";

// Augment vitest's matchers with jest-axe's toHaveNoViolations (registered in
// test/setup.ts via expect.extend).
interface AxeMatchers<R = unknown> {
  toHaveNoViolations(): R;
}

declare module "vitest" {
  interface Assertion<T = unknown> extends AxeMatchers<T> {}
  interface AsymmetricMatchersContaining extends AxeMatchers {}
}
