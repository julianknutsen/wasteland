import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ConfirmDialog } from "./ConfirmDialog";

describe("ConfirmDialog", () => {
  it("renders message", () => {
    render(<ConfirmDialog message="Delete this?" onConfirm={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.getByText("Delete this?")).toBeInTheDocument();
  });

  it("confirm button calls onConfirm", async () => {
    const onConfirm = vi.fn();
    render(<ConfirmDialog message="Sure?" onConfirm={onConfirm} onCancel={vi.fn()} />);
    await userEvent.click(screen.getByText("Confirm"));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("cancel button calls onCancel", async () => {
    const onCancel = vi.fn();
    render(<ConfirmDialog message="Sure?" onConfirm={vi.fn()} onCancel={onCancel} />);
    await userEvent.click(screen.getByText("Cancel"));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("Escape calls onCancel", () => {
    const onCancel = vi.fn();
    render(<ConfirmDialog message="Sure?" onConfirm={vi.fn()} onCancel={onCancel} />);
    fireEvent.keyDown(window, { key: "Escape" });
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("overlay click calls onCancel", async () => {
    const onCancel = vi.fn();
    const { container } = render(<ConfirmDialog message="Sure?" onConfirm={vi.fn()} onCancel={onCancel} />);
    // The overlay is the outermost div
    const overlay = container.firstChild as HTMLElement;
    await userEvent.click(overlay);
    expect(onCancel).toHaveBeenCalled();
  });
});
