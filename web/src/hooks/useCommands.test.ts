import { act, renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { type Command, useCommandRegistry } from "./useCommands";

function makeCmd(id: string): Command {
  return { id, label: id, group: "test", action: () => {} };
}

describe("useCommandRegistry", () => {
  it("register() adds commands and getCommands() returns them", () => {
    const { result } = renderHook(() => useCommandRegistry());
    act(() => {
      result.current.register([makeCmd("a"), makeCmd("b")]);
    });
    expect(result.current.getCommands()).toHaveLength(2);
    expect(result.current.getCommands().map((c) => c.id)).toEqual(["a", "b"]);
  });

  it("unregister function removes commands", () => {
    const { result } = renderHook(() => useCommandRegistry());
    let unsub: () => void;
    act(() => {
      unsub = result.current.register([makeCmd("a")]);
    });
    expect(result.current.getCommands()).toHaveLength(1);
    act(() => {
      unsub();
    });
    expect(result.current.getCommands()).toHaveLength(0);
  });

  it("multiple registrations do not collide", () => {
    const { result } = renderHook(() => useCommandRegistry());
    act(() => {
      result.current.register([makeCmd("a")]);
      result.current.register([makeCmd("b")]);
    });
    expect(result.current.getCommands()).toHaveLength(2);
  });

  it("subscribe() fires on change and returns unsubscribe", () => {
    const { result } = renderHook(() => useCommandRegistry());
    const listener = vi.fn();
    let unsub: () => void;
    act(() => {
      unsub = result.current.subscribe(listener);
    });
    act(() => {
      result.current.register([makeCmd("a")]);
    });
    expect(listener).toHaveBeenCalled();
    listener.mockClear();
    act(() => {
      unsub();
    });
    act(() => {
      result.current.register([makeCmd("b")]);
    });
    expect(listener).not.toHaveBeenCalled();
  });
});
