import { screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { makeDashboardResponse, makeSummary, mockFetch, renderWithRouter } from "../test-utils";
import { Dashboard } from "./Dashboard";

let cleanupFetch: () => void;
afterEach(() => cleanupFetch?.());

describe("Dashboard", () => {
  it("renders three sections", async () => {
    cleanupFetch = mockFetch(() =>
      makeDashboardResponse({
        claimed: [makeSummary({ id: "1", title: "Claimed task" })],
        in_review: [makeSummary({ id: "2", title: "Review task" })],
        completed: [makeSummary({ id: "3", title: "Done task" })],
      }),
    );
    renderWithRouter(<Dashboard />);
    await waitFor(() => expect(screen.getByText("Claimed task")).toBeInTheDocument());
    expect(screen.getByText("Review task")).toBeInTheDocument();
    expect(screen.getByText("Done task")).toBeInTheDocument();
  });

  it("shows empty state per empty section", async () => {
    cleanupFetch = mockFetch(() => makeDashboardResponse());
    renderWithRouter(<Dashboard />);
    await waitFor(() => expect(screen.getByText("No claimed items")).toBeInTheDocument());
    expect(screen.getByText("No in review items")).toBeInTheDocument();
    expect(screen.getByText("No completed items")).toBeInTheDocument();
  });

  it("shows skeleton while loading", () => {
    cleanupFetch = mockFetch(() => new Promise(() => {}));
    renderWithRouter(<Dashboard />);
    expect(screen.getByText("My Dashboard")).toBeInTheDocument();
    expect(screen.getByText("Claimed")).toBeInTheDocument();
  });

  it("shows error on fetch failure", async () => {
    cleanupFetch = mockFetch(() => new Response(JSON.stringify({ error: "dashboard error" }), { status: 500 }));
    renderWithRouter(<Dashboard />);
    await waitFor(() => expect(screen.getByText("dashboard error")).toBeInTheDocument());
  });

  it("section headers show count", async () => {
    cleanupFetch = mockFetch(() =>
      makeDashboardResponse({
        claimed: [makeSummary({ id: "1" }), makeSummary({ id: "2" })],
        in_review: [],
        completed: [makeSummary({ id: "3" })],
      }),
    );
    renderWithRouter(<Dashboard />);
    await waitFor(() => expect(screen.getByText("Claimed (2)")).toBeInTheDocument());
    expect(screen.getByText("In Review (0)")).toBeInTheDocument();
    expect(screen.getByText("Completed (1)")).toBeInTheDocument();
  });
});
