import { render, fireEvent } from "@testing-library/react";
import { axe } from "jest-axe";
import { describe, it, expect } from "vitest";
import { OnboardingDialog } from "@/components/OnboardingDialog";
import { ConfirmDialog } from "@/components/ConfirmDialog";

describe("a11y: OnboardingDialog", () => {
  it("renders role=dialog with aria-modal when open", () => {
    const { container } = render(<OnboardingDialog open={true} onDismiss={() => {}} />);
    const dialog = container.querySelector("[role='dialog']");
    expect(dialog).toBeTruthy();
    expect(dialog?.getAttribute("aria-modal")).toBe("true");
  });

  it("is not rendered when closed", () => {
    const { container } = render(<OnboardingDialog open={false} onDismiss={() => {}} />);
    expect(container.querySelector("[role='dialog']")).toBeNull();
  });

  it("has no axe violations when open", async () => {
    const { container } = render(<OnboardingDialog open={true} onDismiss={() => {}} />);
    const results = await axe(container, { rules: { region: { enabled: false }, "landmark-one-main": { enabled: false }, "page-has-heading-one": { enabled: false } } });
    expect(results).toHaveNoViolations();
  });
});

describe("a11y: ConfirmDialog", () => {
  it("renders role=alertdialog with typed confirmation guard", () => {
    const { container, getByText } = render(
      <ConfirmDialog open={true} title="Delete" confirmText="DELETE" description="Irreversible." actionLabel="Confirm" onConfirm={() => {}} onCancel={() => {}} />,
    );
    expect(container.querySelector("[role='alertdialog']")).toBeTruthy();
    // Button disabled until correct text typed.
    const btn = getByText("Confirm") as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
  });

  it("enables action button only when confirmText matches", () => {
    const { getByText, getByRole } = render(
      <ConfirmDialog open={true} title="Delete" confirmText="DELETE" description="Test" actionLabel="Go" onConfirm={() => {}} onCancel={() => {}} />,
    );
    const input = getByRole("textbox") as HTMLInputElement;
    fireEvent.change(input, { target: { value: "DELETE" } });
    expect((getByText("Go") as HTMLButtonElement).disabled).toBe(false);
  });

  it("has no axe violations", async () => {
    const { container } = render(
      <ConfirmDialog open={true} title="Delete" confirmText="DELETE" description="Test" actionLabel="Go" onConfirm={() => {}} onCancel={() => {}} />,
    );
    const results = await axe(container, { rules: { region: { enabled: false }, "landmark-one-main": { enabled: false }, "page-has-heading-one": { enabled: false } } });
    expect(results).toHaveNoViolations();
  });
});
