import { act, renderHook } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useGlobalShortcuts } from "./useGlobalShortcuts";

function wrapper({ children }: { children: React.ReactNode }) {
  return <MemoryRouter>{children}</MemoryRouter>;
}

const mockNavigate = vi.fn();
vi.mock("react-router-dom", async () => {
  const actual = await vi.importActual("react-router-dom");
  return { ...actual, useNavigate: () => mockNavigate };
});

describe("useGlobalShortcuts", () => {
  let onTogglePalette: () => void;
  let onToggleHelp: () => void;
  let onCreateItem: () => void;

  beforeEach(() => {
    onTogglePalette = vi.fn();
    onToggleHelp = vi.fn();
    onCreateItem = vi.fn();
    mockNavigate.mockClear();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function setup(handlers?: { onCreateItem?: () => void }) {
    return renderHook(
      () =>
        useGlobalShortcuts({
          onTogglePalette,
          onToggleHelp,
          onCreateItem: handlers?.onCreateItem ?? onCreateItem,
        }),
      { wrapper },
    );
  }

  function pressKey(key: string, opts: Partial<KeyboardEventInit> = {}) {
    act(() => {
      window.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, ...opts }));
    });
  }

  it("Cmd+K calls onTogglePalette", () => {
    setup();
    pressKey("k", { metaKey: true });
    expect(onTogglePalette).toHaveBeenCalledTimes(1);
  });

  it("Ctrl+K calls onTogglePalette", () => {
    setup();
    pressKey("k", { ctrlKey: true });
    expect(onTogglePalette).toHaveBeenCalledTimes(1);
  });

  it("Cmd+K works even inside inputs", () => {
    setup();
    const input = document.createElement("input");
    document.body.appendChild(input);
    input.focus();
    act(() => {
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "k", metaKey: true, bubbles: true }));
    });
    expect(onTogglePalette).toHaveBeenCalledTimes(1);
    document.body.removeChild(input);
  });

  it("c calls onCreateItem outside inputs", () => {
    setup();
    pressKey("c");
    expect(onCreateItem).toHaveBeenCalledTimes(1);
  });

  it("c is ignored inside inputs", () => {
    setup();
    const input = document.createElement("input");
    document.body.appendChild(input);
    input.focus();
    act(() => {
      input.dispatchEvent(new KeyboardEvent("keydown", { key: "c", bubbles: true }));
    });
    expect(onCreateItem).not.toHaveBeenCalled();
    document.body.removeChild(input);
  });

  it("? calls onToggleHelp outside inputs", () => {
    setup();
    pressKey("?");
    expect(onToggleHelp).toHaveBeenCalledTimes(1);
  });

  it("g+b navigates to /", () => {
    setup();
    pressKey("g");
    pressKey("b");
    expect(mockNavigate).toHaveBeenCalledWith("/");
  });

  it("g+d navigates to /me", () => {
    setup();
    pressKey("g");
    pressKey("d");
    expect(mockNavigate).toHaveBeenCalledWith("/me");
  });

  it("g+s navigates to /settings", () => {
    setup();
    pressKey("g");
    pressKey("s");
    expect(mockNavigate).toHaveBeenCalledWith("/settings");
  });

  it("g then timeout does not navigate", () => {
    vi.useFakeTimers();
    setup();
    pressKey("g");
    act(() => {
      vi.advanceTimersByTime(600);
    });
    pressKey("b");
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
