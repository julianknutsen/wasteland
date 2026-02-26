import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  ApiError,
  accept,
  applyBranch,
  branchDiff,
  browse,
  claim,
  close,
  config,
  createItem,
  dashboard,
  deleteItem,
  detail,
  discardBranch,
  done,
  reject,
  saveSettings,
  submitPR,
  sync,
  unclaim,
  updateItem,
} from "./client";

let cleanup: () => void;

function mockFetch(handler: (url: string, init?: RequestInit) => Response | object) {
  const original = globalThis.fetch;
  globalThis.fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();
    const result = handler(url, init);
    if (result instanceof Response) return result;
    return new Response(JSON.stringify(result), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  }) as typeof fetch;
  cleanup = () => {
    globalThis.fetch = original;
  };
}

afterEach(() => cleanup?.());

describe("request()", () => {
  it("throws ApiError(0) on network error", async () => {
    mockFetch(() => {
      throw new TypeError("fetch failed");
    });
    await expect(browse()).rejects.toThrow(ApiError);
    await expect(browse()).rejects.toMatchObject({
      status: 0,
      message: "Network error — is the server running?",
    });
  });

  it("throws ApiError with status and parsed error message on 400", async () => {
    mockFetch(() => new Response(JSON.stringify({ error: "bad request" }), { status: 400 }));
    await expect(browse()).rejects.toMatchObject({
      status: 400,
      message: "bad request",
    });
  });

  it("throws ApiError with statusText on non-JSON 500", async () => {
    mockFetch(() => new Response("Internal Server Error", { status: 500, statusText: "Internal Server Error" }));
    await expect(browse()).rejects.toMatchObject({
      status: 500,
      message: "Internal Server Error",
    });
  });

  it("returns parsed JSON on 200", async () => {
    mockFetch(() => ({ items: [] }));
    const result = await browse();
    expect(result).toEqual({ items: [] });
  });
});

describe("buildQuery()", () => {
  beforeEach(() => {
    mockFetch((_url) => ({ items: [] }));
  });

  it("sends empty query for empty filter", async () => {
    await browse({});
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/wanted", undefined);
  });

  it("builds correct query string for full filter", async () => {
    await browse({ status: "open", type: "bug", priority: 1, project: "x", search: "foo", sort: "alpha", limit: 10 });
    const url = vi.mocked(globalThis.fetch).mock.calls[0][0] as string;
    expect(url).toContain("status=open");
    expect(url).toContain("type=bug");
    expect(url).toContain("priority=1");
    expect(url).toContain("project=x");
    expect(url).toContain("search=foo");
    expect(url).toContain("sort=alpha");
    expect(url).toContain("limit=10");
  });

  it("includes priority: 0", async () => {
    await browse({ priority: 0 });
    const url = vi.mocked(globalThis.fetch).mock.calls[0][0] as string;
    expect(url).toContain("priority=0");
  });

  it("omits priority: -1", async () => {
    await browse({ priority: -1 });
    const url = vi.mocked(globalThis.fetch).mock.calls[0][0] as string;
    expect(url).not.toContain("priority");
  });

  it("includes view=all in query string", async () => {
    await browse({ view: "all" });
    const url = vi.mocked(globalThis.fetch).mock.calls[0][0] as string;
    expect(url).toContain("view=all");
  });

  it("omits view=mine (default)", async () => {
    await browse({ view: "mine" });
    const url = vi.mocked(globalThis.fetch).mock.calls[0][0] as string;
    expect(url).not.toContain("view");
  });

  it("includes view=upstream in query string", async () => {
    await browse({ view: "upstream" });
    const url = vi.mocked(globalThis.fetch).mock.calls[0][0] as string;
    expect(url).toContain("view=upstream");
  });
});

describe("API functions", () => {
  beforeEach(() => {
    mockFetch(() => ({ items: [], item: {}, ok: true }));
  });

  it("browse() calls GET /api/wanted", async () => {
    await browse();
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/wanted", undefined);
  });

  it("detail() calls GET /api/wanted/:id", async () => {
    await detail("abc");
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/wanted/abc", undefined);
  });

  it("dashboard() calls GET /api/dashboard", async () => {
    await dashboard();
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/dashboard", undefined);
  });

  it("config() calls GET /api/config", async () => {
    await config();
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/config", undefined);
  });

  it("claim() calls POST /api/wanted/:id/claim", async () => {
    await claim("abc");
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/wanted/abc/claim", { method: "POST" });
  });

  it("unclaim() calls POST /api/wanted/:id/unclaim", async () => {
    await unclaim("abc");
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/wanted/abc/unclaim", { method: "POST" });
  });

  it("reject() calls POST with reason", async () => {
    await reject("abc", "not good");
    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toBe("/api/wanted/abc/reject");
    expect(call[1]?.method).toBe("POST");
    expect(JSON.parse(call[1]?.body as string)).toEqual({ reason: "not good" });
  });

  it("close() calls POST /api/wanted/:id/close", async () => {
    await close("abc");
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/wanted/abc/close", { method: "POST" });
  });

  it("done() calls POST with evidence", async () => {
    await done("abc", "http://evidence");
    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toBe("/api/wanted/abc/done");
    expect(JSON.parse(call[1]?.body as string)).toEqual({ evidence: "http://evidence" });
  });

  it("accept() calls POST with stamp data", async () => {
    await accept("abc", { quality: 5, reliability: 4 });
    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toBe("/api/wanted/abc/accept");
    expect(JSON.parse(call[1]?.body as string)).toEqual({ quality: 5, reliability: 4 });
  });

  it("deleteItem() calls DELETE /api/wanted/:id", async () => {
    await deleteItem("abc");
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/wanted/abc", { method: "DELETE" });
  });

  it("submitPR() calls POST /api/branches/pr/:branch", async () => {
    mockFetch(() => ({ url: "https://dolthub.com/pr/1" }));
    const result = await submitPR("wl/fix");
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/branches/pr/wl/fix", { method: "POST" });
    expect(result.url).toBe("https://dolthub.com/pr/1");
  });

  it("applyBranch() calls POST /api/branches/apply/:branch", async () => {
    await applyBranch("wl/fix");
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/branches/apply/wl/fix", { method: "POST" });
  });

  it("discardBranch() calls DELETE /api/branches/:branch", async () => {
    await discardBranch("wl/fix");
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/branches/wl/fix", { method: "DELETE" });
  });

  it("branchDiff() calls GET /api/branches/diff/:branch", async () => {
    mockFetch(() => ({ diff: "+line" }));
    const result = await branchDiff("wl/fix");
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/branches/diff/wl/fix", undefined);
    expect(result.diff).toBe("+line");
  });

  it("sync() calls POST /api/sync", async () => {
    await sync();
    expect(vi.mocked(globalThis.fetch)).toHaveBeenCalledWith("/api/sync", { method: "POST" });
  });

  it("createItem() calls POST /api/wanted with body", async () => {
    await createItem({ title: "New item" });
    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toBe("/api/wanted");
    expect(call[1]?.method).toBe("POST");
    expect(JSON.parse(call[1]?.body as string)).toEqual({ title: "New item" });
  });

  it("updateItem() calls PATCH /api/wanted/:id with body", async () => {
    await updateItem("abc", { title: "Updated" });
    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toBe("/api/wanted/abc");
    expect(call[1]?.method).toBe("PATCH");
    expect(JSON.parse(call[1]?.body as string)).toEqual({ title: "Updated" });
  });

  it("saveSettings() calls PUT /api/settings with body", async () => {
    await saveSettings({ mode: "pr", signing: true });
    const call = vi.mocked(globalThis.fetch).mock.calls[0];
    expect(call[0]).toBe("/api/settings");
    expect(call[1]?.method).toBe("PUT");
    expect(JSON.parse(call[1]?.body as string)).toEqual({ mode: "pr", signing: true });
  });
});
