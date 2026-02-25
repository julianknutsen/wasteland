import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { EmptyState } from "./EmptyState";

describe("EmptyState", () => {
  it("renders title and description", () => {
    render(<EmptyState title="Nothing here" description="Start by creating something" />);
    expect(screen.getByText("Nothing here")).toBeInTheDocument();
    expect(screen.getByText("Start by creating something")).toBeInTheDocument();
  });

  it("renders CTA button when ctaLabel and onCta provided", async () => {
    const onCta = vi.fn();
    render(<EmptyState title="Empty" description="desc" ctaLabel="Create" onCta={onCta} />);
    const btn = screen.getByText("Create");
    expect(btn).toBeInTheDocument();
    await userEvent.click(btn);
    expect(onCta).toHaveBeenCalledTimes(1);
  });

  it("does not render CTA when ctaLabel is missing", () => {
    render(<EmptyState title="Empty" description="desc" />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("does not render CTA when onCta is missing", () => {
    render(<EmptyState title="Empty" description="desc" ctaLabel="Go" />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
