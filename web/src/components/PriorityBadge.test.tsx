import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { PriorityBadge } from "./PriorityBadge";

describe("PriorityBadge", () => {
  it("renders P{n} format", () => {
    render(<PriorityBadge priority={2} />);
    expect(screen.getByText("P2")).toBeInTheDocument();
  });

  it("sets data-priority attribute", () => {
    render(<PriorityBadge priority={0} />);
    expect(screen.getByText("P0")).toHaveAttribute("data-priority", "0");
  });

  it("renders priority 4", () => {
    render(<PriorityBadge priority={4} />);
    expect(screen.getByText("P4")).toBeInTheDocument();
  });
});
