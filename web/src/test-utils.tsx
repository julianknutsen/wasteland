import { type RenderOptions, render } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import type {
  BrowseResponse,
  ConfigResponse,
  DashboardResponse,
  DetailResponse,
  WantedItem,
  WantedSummary,
} from "./api/types";
import { type Command, CommandsContext } from "./hooks/useCommands";

const defaultCommands = {
  commands: [] as Command[],
  register: () => () => {},
};

export function renderWithRouter(ui: React.ReactElement, options?: RenderOptions & { route?: string }) {
  const { route = "/", ...rest } = options ?? {};
  return render(
    <MemoryRouter initialEntries={[route]}>
      <CommandsContext.Provider value={defaultCommands}>{ui}</CommandsContext.Provider>
    </MemoryRouter>,
    rest,
  );
}

export function mockFetch(handler: (url: string, init?: RequestInit) => unknown) {
  const original = globalThis.fetch;
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    const result = await handler(url, init);
    if (result instanceof Response) return result;
    return new Response(JSON.stringify(result), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  }) as typeof fetch;
  return () => {
    globalThis.fetch = original;
  };
}

export function makeItem(overrides: Partial<WantedItem> = {}): WantedItem {
  return {
    id: "item-1",
    title: "Fix the thing",
    priority: 2,
    status: "open",
    effort_level: "medium",
    posted_by: "alice",
    type: "bug",
    ...overrides,
  };
}

export function makeSummary(overrides: Partial<WantedSummary> = {}): WantedSummary {
  return {
    id: "item-1",
    title: "Fix the thing",
    priority: 2,
    status: "open",
    effort_level: "medium",
    posted_by: "alice",
    type: "bug",
    ...overrides,
  };
}

export function makeDetailResponse(overrides: Partial<DetailResponse> = {}): DetailResponse {
  return {
    item: makeItem(),
    actions: ["claim"],
    branch_actions: [],
    mode: "wild-west",
    ...overrides,
  };
}

export function makeBrowseResponse(items: WantedSummary[] = [makeSummary()]): BrowseResponse {
  return { items };
}

export function makeDashboardResponse(overrides: Partial<DashboardResponse> = {}): DashboardResponse {
  return {
    claimed: [],
    in_review: [],
    completed: [],
    ...overrides,
  };
}

export function makeConfigResponse(overrides: Partial<ConfigResponse> = {}): ConfigResponse {
  return {
    rig_handle: "alice",
    mode: "wild-west",
    ...overrides,
  };
}
