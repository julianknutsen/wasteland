import { act, renderHook } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { useFilterParams } from "./useFilterParams";

function wrapper({ children }: { children: React.ReactNode }) {
  return <MemoryRouter>{children}</MemoryRouter>;
}

function wrapperWithRoute(route: string) {
  return ({ children }: { children: React.ReactNode }) => (
    <MemoryRouter initialEntries={[route]}>{children}</MemoryRouter>
  );
}

describe("useFilterParams", () => {
  it("returns empty filter for no params", () => {
    const { result } = renderHook(() => useFilterParams(), { wrapper });
    const [filter] = result.current;
    expect(filter).toEqual({});
  });

  it("parses URL params into BrowseFilter", () => {
    const { result } = renderHook(() => useFilterParams(), {
      wrapper: wrapperWithRoute("/?status=open&type=bug&sort=alpha&search=hello"),
    });
    const [filter] = result.current;
    expect(filter.status).toBe("open");
    expect(filter.type).toBe("bug");
    expect(filter.sort).toBe("alpha");
    expect(filter.search).toBe("hello");
  });

  it("parses priority as number", () => {
    const { result } = renderHook(() => useFilterParams(), {
      wrapper: wrapperWithRoute("/?priority=3"),
    });
    const [filter] = result.current;
    expect(filter.priority).toBe(3);
  });

  it("missing params are undefined", () => {
    const { result } = renderHook(() => useFilterParams(), { wrapper });
    const [filter] = result.current;
    expect(filter.status).toBeUndefined();
    expect(filter.type).toBeUndefined();
    expect(filter.priority).toBeUndefined();
  });

  it("setFilter updates URL params", () => {
    const { result } = renderHook(() => useFilterParams(), { wrapper });
    act(() => {
      const [, setFilter] = result.current;
      setFilter({ status: "claimed", type: "feature" });
    });
    const [filter] = result.current;
    expect(filter.status).toBe("claimed");
    expect(filter.type).toBe("feature");
  });

  it("default sort 'priority' is omitted from URL", () => {
    const { result } = renderHook(() => useFilterParams(), { wrapper });
    act(() => {
      const [, setFilter] = result.current;
      setFilter({ sort: "priority" });
    });
    const [filter] = result.current;
    // sort "priority" is not serialized to URL, so it should be undefined when re-parsed
    expect(filter.sort).toBeUndefined();
  });
});
