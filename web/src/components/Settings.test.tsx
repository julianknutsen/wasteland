import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { makeConfigResponse, mockFetch, renderWithRouter } from "../test-utils";
import { Settings } from "./Settings";

type FetchFn = (url: string, init?: RequestInit) => object;

let cleanupFetch: () => void;
afterEach(() => cleanupFetch?.());

describe("Settings", () => {
  it("loads config on mount", async () => {
    cleanupFetch = mockFetch((url) => {
      if (url.includes("/api/config")) return makeConfigResponse({ rig_handle: "alice", mode: "pr" });
      return {};
    });
    renderWithRouter(<Settings />);
    expect(screen.getByText("Loading...")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByText("alice")).toBeInTheDocument());
  });

  it("save calls saveSettings()", async () => {
    const fetchFn = vi.fn<FetchFn>((url) => {
      if (url.includes("/api/config")) return makeConfigResponse();
      return {};
    });
    cleanupFetch = mockFetch(fetchFn);
    renderWithRouter(<Settings />);
    await waitFor(() => expect(screen.getByText("Save")).toBeInTheDocument());
    await userEvent.click(screen.getByText("Save"));
    await waitFor(() => {
      const saveCalls = fetchFn.mock.calls.filter(([u]) => u.includes("/api/settings"));
      expect(saveCalls.length).toBeGreaterThan(0);
      expect(saveCalls[0][1]?.method).toBe("PUT");
    });
  });

  it("sync calls sync()", async () => {
    const fetchFn = vi.fn<FetchFn>((url) => {
      if (url.includes("/api/config")) return makeConfigResponse();
      return {};
    });
    cleanupFetch = mockFetch(fetchFn);
    renderWithRouter(<Settings />);
    await waitFor(() => expect(screen.getByText("Sync")).toBeInTheDocument());
    await userEvent.click(screen.getByText("Sync"));
    await waitFor(() => {
      const syncCalls = fetchFn.mock.calls.filter(([u]) => u.includes("/api/sync"));
      expect(syncCalls.length).toBeGreaterThan(0);
    });
  });

  it("shows saving state", async () => {
    let resolveSettings: (v: unknown) => void;
    cleanupFetch = mockFetch((url) => {
      if (url.includes("/api/config")) return makeConfigResponse();
      if (url.includes("/api/settings"))
        return new Promise((r) => {
          resolveSettings = r;
        });
      return {};
    });
    renderWithRouter(<Settings />);
    await waitFor(() => expect(screen.getByText("Save")).toBeInTheDocument());
    await userEvent.click(screen.getByText("Save"));
    expect(screen.getByText("Saving...")).toBeInTheDocument();
    resolveSettings!({});
    await waitFor(() => expect(screen.getByText("Save")).toBeInTheDocument());
  });
});
