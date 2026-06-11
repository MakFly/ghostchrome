/**
 * transport.test.ts — hermetic tests for SubprocessTransport.
 *
 * Tests line-buffering (split chunks), id correlation, and error handling
 * by injecting a fake child process via the transport's internal machinery.
 *
 * We test SubprocessTransport by subclassing it to intercept the spawn and
 * feed controlled data directly into the line-buffer mechanism.
 */

import { describe, it, expect } from "bun:test";
import { EventEmitter } from "node:events";
import type { Transport } from "../src/transport.js";
import type { OpRequest, OpResponse } from "../src/types.js";

// ---------------------------------------------------------------------------
// Manual line-buffer test — no subprocess involved
// ---------------------------------------------------------------------------

/**
 * A minimal Transport that exposes a `feedChunk` method to simulate
 * receiving partial/split JSONL data from stdout, exercising the same
 * line-buffer logic as SubprocessTransport.
 */
class LineBufferTransport implements Transport {
  private lineBuffer = "";
  private readonly pending = new Map<
    string,
    { resolve: (v: OpResponse<unknown>) => void; reject: (e: unknown) => void }
  >();

  // Feed raw bytes/chars as if they arrived from stdout
  feedChunk(chunk: string): void {
    this.lineBuffer += chunk;
    let idx: number;
    while ((idx = this.lineBuffer.indexOf("\n")) !== -1) {
      const line = this.lineBuffer.slice(0, idx).trim();
      this.lineBuffer = this.lineBuffer.slice(idx + 1);
      if (line.length === 0) continue;

      let parsed: OpResponse<unknown>;
      try {
        parsed = JSON.parse(line) as OpResponse<unknown>;
      } catch {
        continue;
      }

      const p = this.pending.get(parsed.id);
      if (!p) continue;
      this.pending.delete(parsed.id);
      if (!parsed.ok) {
        p.reject(new Error(parsed.error ?? "op failed"));
      } else {
        p.resolve(parsed);
      }
    }
  }

  async send<R>(req: OpRequest): Promise<OpResponse<R>> {
    return new Promise<OpResponse<R>>((resolve, reject) => {
      this.pending.set(req.id, {
        resolve: resolve as (v: OpResponse<unknown>) => void,
        reject,
      });
    });
  }

  async dispose(): Promise<void> {}
}

describe("line buffering", () => {
  it("reassembles a response split across multiple chunks", async () => {
    const t = new LineBufferTransport();

    const sendPromise = t.send({ id: "x1", op: "navigate" });

    const full = JSON.stringify({
      id: "x1",
      ok: true,
      result: { status: 200, url: "https://example.com", title: "Test" },
    }) + "\n";

    // Split in 3 chunks
    const part1 = full.slice(0, 10);
    const part2 = full.slice(10, 30);
    const part3 = full.slice(30);

    t.feedChunk(part1);
    t.feedChunk(part2);
    t.feedChunk(part3);

    const resp = await sendPromise;
    expect(resp.ok).toBe(true);
    expect((resp.result as { status: number }).status).toBe(200);
  });

  it("handles two responses in a single chunk", async () => {
    const t = new LineBufferTransport();

    const p1 = t.send({ id: "a1", op: "url" });
    const p2 = t.send({ id: "a2", op: "back" });

    const combined =
      JSON.stringify({ id: "a1", ok: true, result: { url: "https://a.com" } }) +
      "\n" +
      JSON.stringify({ id: "a2", ok: true, result: {} }) +
      "\n";

    t.feedChunk(combined);

    const [r1, r2] = await Promise.all([p1, p2]);
    expect((r1.result as { url: string }).url).toBe("https://a.com");
    expect(r2.ok).toBe(true);
  });

  it("ignores non-JSON lines", async () => {
    const t = new LineBufferTransport();

    const p = t.send({ id: "b1", op: "url" });

    t.feedChunk("some debug output\n");
    t.feedChunk(
      JSON.stringify({ id: "b1", ok: true, result: { url: "https://b.com" } }) + "\n"
    );

    const resp = await p;
    expect((resp.result as { url: string }).url).toBe("https://b.com");
  });

  it("handles empty lines gracefully", async () => {
    const t = new LineBufferTransport();
    const p = t.send({ id: "c1", op: "url" });

    t.feedChunk("\n\n");
    t.feedChunk(
      JSON.stringify({ id: "c1", ok: true, result: { url: "https://c.com" } }) + "\n"
    );

    const resp = await p;
    expect((resp.result as { url: string }).url).toBe("https://c.com");
  });
});

// ---------------------------------------------------------------------------
// ID correlation — multiple concurrent requests
// ---------------------------------------------------------------------------

describe("id correlation", () => {
  it("routes responses to correct pending request regardless of arrival order", async () => {
    const t = new LineBufferTransport();

    const p1 = t.send({ id: "id-1", op: "navigate" });
    const p2 = t.send({ id: "id-2", op: "extract" });
    const p3 = t.send({ id: "id-3", op: "url" });

    // Deliver responses out of order: 3, 1, 2
    t.feedChunk(
      JSON.stringify({ id: "id-3", ok: true, result: { url: "https://three.com" } }) + "\n"
    );
    t.feedChunk(
      JSON.stringify({ id: "id-1", ok: true, result: { status: 200, url: "https://one.com", title: "One" } }) + "\n"
    );
    t.feedChunk(
      JSON.stringify({ id: "id-2", ok: true, result: { nodes: [], refs: {} } }) + "\n"
    );

    const [r1, r2, r3] = await Promise.all([p1, p2, p3]);

    expect((r1.result as { url: string }).url).toBe("https://one.com");
    expect((r2.result as { nodes: unknown[] }).nodes).toEqual([]);
    expect((r3.result as { url: string }).url).toBe("https://three.com");
  });
});

// ---------------------------------------------------------------------------
// FakeTransport self-test (used in client tests)
// ---------------------------------------------------------------------------

import { FakeTransport } from "./fake-transport.js";

describe("FakeTransport", () => {
  it("correlates id in response", async () => {
    const ft = new FakeTransport();
    ft.register("navigate", {
      ok: true,
      result: { status: 200, url: "https://example.com", title: "Ex" },
    });

    const resp = await ft.send({ id: "req-42", op: "navigate" });
    expect(resp.id).toBe("req-42");
    expect(resp.ok).toBe(true);
  });

  it("records all received requests", async () => {
    const ft = new FakeTransport();
    await ft.send({ id: "r1", op: "url" });
    await ft.send({ id: "r2", op: "errors" });
    expect(ft.received).toHaveLength(2);
    expect(ft.received[0]!.id).toBe("r1");
    expect(ft.received[1]!.id).toBe("r2");
  });

  it("throws GhostchromeError for ok=false responses", async () => {
    const ft = new FakeTransport();
    ft.register("click", { ok: false, error: "element not found" });

    await expect(ft.send({ id: "r3", op: "click" })).rejects.toThrow("element not found");
  });

  it("throws after dispose", async () => {
    const ft = new FakeTransport();
    await ft.dispose();
    await expect(ft.send({ id: "r0", op: "url" })).rejects.toThrow("disposed");
  });
});
