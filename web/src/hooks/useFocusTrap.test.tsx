import { fireEvent, render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useFocusTrap } from "./useFocusTrap";

function TrapHarness({ active }: { active: boolean }) {
  const ref = useFocusTrap(active);
  return (
    <div ref={ref}>
      <button type="button" data-testid="first">
        First
      </button>
      <button type="button" data-testid="second">
        Second
      </button>
      <button type="button" data-testid="third">
        Third
      </button>
    </div>
  );
}

describe("useFocusTrap", () => {
  it("focuses first element when active", () => {
    const { getByTestId } = render(<TrapHarness active={true} />);
    expect(document.activeElement).toBe(getByTestId("first"));
  });

  it("Tab wraps from last to first", () => {
    const { getByTestId } = render(<TrapHarness active={true} />);
    getByTestId("third").focus();
    fireEvent.keyDown(getByTestId("third").parentElement!, { key: "Tab" });
    expect(document.activeElement).toBe(getByTestId("first"));
  });

  it("Shift+Tab wraps from first to last", () => {
    const { getByTestId } = render(<TrapHarness active={true} />);
    getByTestId("first").focus();
    fireEvent.keyDown(getByTestId("first").parentElement!, { key: "Tab", shiftKey: true });
    expect(document.activeElement).toBe(getByTestId("third"));
  });

  it("does not trap when active=false", () => {
    const { getByTestId } = render(<TrapHarness active={false} />);
    // When not active, it should NOT auto-focus the first element
    expect(document.activeElement).not.toBe(getByTestId("first"));
  });
});
