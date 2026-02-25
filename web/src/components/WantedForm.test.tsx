import { fireEvent, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { WantedItem } from "../api/types";
import { mockFetch, renderWithRouter } from "../test-utils";
import { WantedForm } from "./WantedForm";

const onClose = vi.fn();
const onSaved = vi.fn();
let cleanupFetch: () => void;

afterEach(() => {
  cleanupFetch?.();
  vi.clearAllMocks();
});

function makeEditItem(): WantedItem {
  return {
    id: "item-1",
    title: "Existing Title",
    description: "Existing desc",
    project: "myproj",
    type: "bug",
    priority: 1,
    effort_level: "small",
    tags: ["tag1", "tag2"],
    status: "open",
    posted_by: "alice",
  };
}

type FetchFn = (url: string, init?: RequestInit) => object;

describe("WantedForm", () => {
  it("create mode: renders empty form", () => {
    cleanupFetch = mockFetch(() => ({}));
    renderWithRouter(<WantedForm onClose={onClose} onSaved={onSaved} />);
    expect(screen.getByText("Post New Item")).toBeInTheDocument();
    const titleInput = screen.getByPlaceholderText("What needs to be done?");
    expect(titleInput).toHaveValue("");
  });

  it("create mode: calls createItem on submit", async () => {
    const fetchFn = vi.fn<FetchFn>(() => ({ detail: null }));
    cleanupFetch = mockFetch(fetchFn);
    renderWithRouter(<WantedForm onClose={onClose} onSaved={onSaved} />);

    await userEvent.type(screen.getByPlaceholderText("What needs to be done?"), "New task");
    await userEvent.click(screen.getByText("Post"));
    await waitFor(() => {
      const postCalls = fetchFn.mock.calls.filter(([, init]) => init?.method === "POST");
      expect(postCalls.length).toBeGreaterThan(0);
      expect(postCalls[0][0]).toBe("/api/wanted");
    });
  });

  it("edit mode: pre-populates fields", () => {
    cleanupFetch = mockFetch(() => ({}));
    renderWithRouter(<WantedForm item={makeEditItem()} onClose={onClose} onSaved={onSaved} />);
    expect(screen.getByText("Edit Item")).toBeInTheDocument();
    expect(screen.getByPlaceholderText("What needs to be done?")).toHaveValue("Existing Title");
    expect(screen.getByPlaceholderText("Details, context, acceptance criteria...")).toHaveValue("Existing desc");
  });

  it("edit mode: calls updateItem on submit", async () => {
    const fetchFn = vi.fn<FetchFn>(() => ({ detail: null }));
    cleanupFetch = mockFetch(fetchFn);
    renderWithRouter(<WantedForm item={makeEditItem()} onClose={onClose} onSaved={onSaved} />);

    await userEvent.click(screen.getByText("Update"));
    await waitFor(() => {
      const patchCalls = fetchFn.mock.calls.filter(([, init]) => init?.method === "PATCH");
      expect(patchCalls.length).toBeGreaterThan(0);
      expect(patchCalls[0][0]).toBe("/api/wanted/item-1");
    });
  });

  it("parses tags from comma-separated input", async () => {
    const fetchFn = vi.fn<FetchFn>(() => ({ detail: null }));
    cleanupFetch = mockFetch(fetchFn);
    renderWithRouter(<WantedForm onClose={onClose} onSaved={onSaved} />);

    await userEvent.type(screen.getByPlaceholderText("What needs to be done?"), "Tagged task");
    await userEvent.type(screen.getByPlaceholderText("tag1, tag2, ..."), " foo , bar , , baz ");
    await userEvent.click(screen.getByText("Post"));
    await waitFor(() => {
      const postCalls = fetchFn.mock.calls.filter(([, init]) => init?.method === "POST");
      const body = JSON.parse(postCalls[0][1]?.body as string);
      expect(body.tags).toEqual(["foo", "bar", "baz"]);
    });
  });

  it("empty title disables submit", () => {
    cleanupFetch = mockFetch(() => ({}));
    renderWithRouter(<WantedForm onClose={onClose} onSaved={onSaved} />);
    expect(screen.getByText("Post")).toBeDisabled();
  });

  it("Escape closes the form", () => {
    cleanupFetch = mockFetch(() => ({}));
    renderWithRouter(<WantedForm onClose={onClose} onSaved={onSaved} />);
    fireEvent.keyDown(window, { key: "Escape" });
    expect(onClose).toHaveBeenCalled();
  });

  it("Cmd+Enter submits", async () => {
    const fetchFn = vi.fn<FetchFn>(() => ({ detail: null }));
    cleanupFetch = mockFetch(fetchFn);
    renderWithRouter(<WantedForm onClose={onClose} onSaved={onSaved} />);
    await userEvent.type(screen.getByPlaceholderText("What needs to be done?"), "Task via shortcut");
    fireEvent.keyDown(window, { key: "Enter", metaKey: true });
    await waitFor(() => {
      const postCalls = fetchFn.mock.calls.filter(([, init]) => init?.method === "POST");
      expect(postCalls.length).toBeGreaterThan(0);
    });
  });
});
