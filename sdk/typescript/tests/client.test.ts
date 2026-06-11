/**
 * client.test.ts — hermetic tests for the Ghostchrome client.
 *
 * Uses FakeTransport; no real Chrome or binary required.
 * Canned responses use REAL shapes measured from the live binary.
 */

import { describe, it, expect } from "bun:test";
import { Ghostchrome } from "../src/client.js";
import { FakeTransport } from "./fake-transport.js";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeClient(): { gc: Ghostchrome; transport: FakeTransport } {
  const transport = new FakeTransport();
  const gc = new Ghostchrome({ transport });
  return { gc, transport };
}

// ---------------------------------------------------------------------------
// ID correlation
// ---------------------------------------------------------------------------

describe("id correlation", () => {
  it("sends unique ids and correlates responses", async () => {
    const { gc, transport } = makeClient();
    transport.register("navigate", {
      ok: true,
      result: { status: 200, url: "https://example.com", title: "Example", time_ms: 42 },
    });

    const p1 = gc.navigate("https://example.com");
    const p2 = gc.navigate("https://other.com");

    const [r1, r2] = await Promise.all([p1, p2]);

    expect(r1.result.url).toBe("https://example.com");
    expect(r2.result.url).toBe("https://example.com"); // canned, same response
    expect(transport.received).toHaveLength(2);
    expect(transport.received[0]!.id).not.toBe(transport.received[1]!.id);
  });
});

// ---------------------------------------------------------------------------
// navigate
// ---------------------------------------------------------------------------

describe("navigate", () => {
  it("sends correct op and args", async () => {
    const { gc, transport } = makeClient();
    transport.register("navigate", {
      ok: true,
      result: { status: 200, url: "https://example.com", title: "Example Domain", time_ms: 20 },
    });

    const { result, observation } = await gc.navigate("https://example.com", {
      wait: "load",
    });

    const req = transport.received[0]!;
    expect(req.op).toBe("navigate");
    expect((req.args as { url: string }).url).toBe("https://example.com");
    expect((req.args as { wait?: string }).wait).toBe("load");

    expect(result.status).toBe(200);
    expect(result.url).toBe("https://example.com");
    expect(result.title).toBe("Example Domain");
    expect(result.time_ms).toBe(20);
    expect(observation).toBeUndefined();
  });

  it("includes observation when present", async () => {
    const { gc, transport } = makeClient();
    transport.register("navigate", {
      ok: true,
      result: { status: 200, url: "https://example.com", title: "Example", time_ms: 15 },
      observation: { url: "https://example.com", a11y_diff: "added 5 nodes" },
    });

    const { observation } = await gc.navigate("https://example.com");
    expect(observation?.a11y_diff).toBe("added 5 nodes");
  });
});

// ---------------------------------------------------------------------------
// extract
// ---------------------------------------------------------------------------

describe("extract", () => {
  it("sends op with level and selector, stats use correct field names", async () => {
    const { gc, transport } = makeClient();
    transport.register("extract", {
      ok: true,
      result: {
        nodes: [{ role: "button", name: "Submit", ref: "@1" }],
        refs: { "@1": { role: "button", name: "Submit" } },
        // Correct field names: total_nodes / filtered_nodes / interactive_count
        stats: { total_nodes: 15, filtered_nodes: 1, interactive_count: 1 },
      },
    });

    const { result } = await gc.extract({ level: "skeleton", selector: "#main" });

    const req = transport.received[0]!;
    expect(req.op).toBe("extract");
    expect((req.args as { level: string }).level).toBe("skeleton");
    expect((req.args as { selector: string }).selector).toBe("#main");

    expect(result.refs["@1"]?.role).toBe("button");
    expect(result.stats.total_nodes).toBe(15);
    expect(result.stats.filtered_nodes).toBe(1);
    expect(result.stats.interactive_count).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// click
// ---------------------------------------------------------------------------

describe("click", () => {
  it("sends ref as arg, result is empty (binary omits result)", async () => {
    const { gc, transport } = makeClient();
    // click emits no result — binary omits the field entirely
    transport.register("click", { ok: true });

    await gc.click("@3");

    const req = transport.received[0]!;
    expect(req.op).toBe("click");
    expect((req.args as { ref: string }).ref).toBe("@3");
  });
});

// ---------------------------------------------------------------------------
// type
// ---------------------------------------------------------------------------

describe("type", () => {
  it("sends ref and text", async () => {
    const { gc, transport } = makeClient();
    transport.register("type", { ok: true });

    await gc.type("@2", "hello world");

    const req = transport.received[0]!;
    expect(req.op).toBe("type");
    expect((req.args as { ref: string }).ref).toBe("@2");
    expect((req.args as { text: string }).text).toBe("hello world");
  });
});

// ---------------------------------------------------------------------------
// press
// ---------------------------------------------------------------------------

describe("press", () => {
  it("sends key", async () => {
    const { gc, transport } = makeClient();
    transport.register("press", { ok: true });

    await gc.press("Enter");
    expect((transport.received[0]!.args as { key: string }).key).toBe("Enter");
  });

  it("sends key + ref when ref provided", async () => {
    const { gc, transport } = makeClient();
    transport.register("press", { ok: true });

    await gc.press("Escape", { ref: "@5" });
    const args = transport.received[0]!.args as { key: string; ref: string };
    expect(args.key).toBe("Escape");
    expect(args.ref).toBe("@5");
  });
});

// ---------------------------------------------------------------------------
// select
// ---------------------------------------------------------------------------

describe("select", () => {
  it("sends ref and values array", async () => {
    const { gc, transport } = makeClient();
    transport.register("select", { ok: true });

    await gc.select("@7", ["option-a", "option-b"]);
    const args = transport.received[0]!.args as { ref: string; values: string[] };
    expect(args.ref).toBe("@7");
    expect(args.values).toEqual(["option-a", "option-b"]);
  });
});

// ---------------------------------------------------------------------------
// fill
// ---------------------------------------------------------------------------

describe("fill", () => {
  it("sends fields map and returns filled count", async () => {
    const { gc, transport } = makeClient();
    // fill returns { filled: number } — NOT empty
    transport.register("fill", { ok: true, result: { filled: 2 } });

    const { result } = await gc.fill({ "@1": "Alice", "@2": "alice@example.com" });
    const args = transport.received[0]!.args as { fields: Record<string, string> };
    expect(args.fields["@1"]).toBe("Alice");
    expect(args.fields["@2"]).toBe("alice@example.com");
    expect(result.filled).toBe(2);
  });
});

// ---------------------------------------------------------------------------
// hover
// ---------------------------------------------------------------------------

describe("hover", () => {
  it("sends ref", async () => {
    const { gc, transport } = makeClient();
    transport.register("hover", { ok: true });

    await gc.hover("@4");
    expect((transport.received[0]!.args as { ref: string }).ref).toBe("@4");
    expect(transport.received[0]!.op).toBe("hover");
  });
});

// ---------------------------------------------------------------------------
// scrollBy / scrollTo
// ---------------------------------------------------------------------------

describe("scrollBy", () => {
  it("sends dy and returns final y position", async () => {
    const { gc, transport } = makeClient();
    // scroll_by returns { y: number }
    transport.register("scroll_by", { ok: true, result: { y: 300 } });

    const { result } = await gc.scrollBy(300);
    const args = transport.received[0]!.args as { dy: number };
    expect(args.dy).toBe(300);
    expect(result.y).toBe(300);
  });
});

describe("scrollTo", () => {
  it("sends y and returns final y position", async () => {
    const { gc, transport } = makeClient();
    // scroll_to returns { y: number }
    transport.register("scroll_to", { ok: true, result: { y: 1000 } });

    const { result } = await gc.scrollTo({ y: 1000 });
    const args = transport.received[0]!.args as { y: number };
    expect(args.y).toBe(1000);
    expect(result.y).toBe(1000);
  });

  it("sends bottom=true", async () => {
    const { gc, transport } = makeClient();
    transport.register("scroll_to", { ok: true, result: { y: 9999 } });

    await gc.scrollTo({ bottom: true });
    const args = transport.received[0]!.args as { bottom: boolean };
    expect(args.bottom).toBe(true);
  });
});

// ---------------------------------------------------------------------------
// eval
// ---------------------------------------------------------------------------

describe("eval", () => {
  it("sends expr and parses string value", async () => {
    const { gc, transport } = makeClient();
    // eval always returns { value: string } — binary stringifies
    transport.register("eval", { ok: true, result: { value: "2" } });

    const { result } = await gc.eval("1 + 1");
    expect((transport.received[0]!.args as { expr: string }).expr).toBe("1 + 1");
    expect(result.value).toBe("2");
  });

  it("sends expr with ref", async () => {
    const { gc, transport } = makeClient();
    transport.register("eval", { ok: true, result: { value: "hello" } });

    await gc.eval("this.textContent", { ref: "@2" });
    const args = transport.received[0]!.args as { expr: string; ref: string };
    expect(args.expr).toBe("this.textContent");
    expect(args.ref).toBe("@2");
  });
});

// ---------------------------------------------------------------------------
// screenshot
// ---------------------------------------------------------------------------

describe("screenshot", () => {
  it("sends full_page flag, result has base64 and mime", async () => {
    const { gc, transport } = makeClient();
    // screenshot returns { base64: string, mime: string } — NOT { path? }
    transport.register("screenshot", {
      ok: true,
      result: { base64: "abc123", mime: "image/png" },
    });

    const { result } = await gc.screenshot({ full_page: true });
    const args = transport.received[0]!.args as { full_page: boolean };
    expect(args.full_page).toBe(true);
    expect(result.base64).toBe("abc123");
    expect(result.mime).toBe("image/png");
  });
});

// ---------------------------------------------------------------------------
// wait
// ---------------------------------------------------------------------------

describe("wait", () => {
  it("sends selector, result is empty (binary omits result)", async () => {
    const { gc, transport } = makeClient();
    transport.register("wait", { ok: true });

    await gc.wait({ selector: "#loaded" });
    expect((transport.received[0]!.args as { selector: string }).selector).toBe("#loaded");
  });

  it("sends ms delay", async () => {
    const { gc, transport } = makeClient();
    transport.register("wait", { ok: true });

    await gc.wait({ ms: 500 });
    expect((transport.received[0]!.args as { ms: number }).ms).toBe(500);
  });
});

// ---------------------------------------------------------------------------
// errors / url
// ---------------------------------------------------------------------------

describe("errors", () => {
  it("returns top-level array of ErrorEntry (NOT wrapped in {errors:[]})", async () => {
    const { gc, transport } = makeClient();
    // errors() result is a TOP-LEVEL ARRAY — not {errors:[...]}
    transport.register("errors", {
      ok: true,
      result: [
        {
          type: "console",
          level: "error",
          message: "Uncaught TypeError",
          source: "app.js",
          time_ms: 123,
        },
      ],
    });

    const { result } = await gc.errors();
    expect(Array.isArray(result)).toBe(true);
    expect(result).toHaveLength(1);
    expect(result[0]!.message).toBe("Uncaught TypeError");
    expect(result[0]!.level).toBe("error");
    expect(result[0]!.time_ms).toBe(123);
  });

  it("returns empty array when no errors", async () => {
    const { gc, transport } = makeClient();
    transport.register("errors", { ok: true, result: [] });

    const { result } = await gc.errors();
    expect(Array.isArray(result)).toBe(true);
    expect(result).toHaveLength(0);
  });
});

describe("url", () => {
  it("returns current url and required title", async () => {
    const { gc, transport } = makeClient();
    // title is now required in UrlResult
    transport.register("url", {
      ok: true,
      result: { url: "https://example.com/page", title: "Page" },
    });

    const { result } = await gc.url();
    expect(result.url).toBe("https://example.com/page");
    expect(result.title).toBe("Page");
  });
});

// ---------------------------------------------------------------------------
// back / forward
// ---------------------------------------------------------------------------

describe("back / forward", () => {
  it("back sends back op, result may have url and title", async () => {
    const { gc, transport } = makeClient();
    transport.register("back", { ok: true, result: { url: "https://example.com" } });

    const { result } = await gc.back();
    expect(transport.received[0]!.op).toBe("back");
    expect(result.url).toBe("https://example.com");
  });

  it("back result may be {} when no history", async () => {
    const { gc, transport } = makeClient();
    // Back with no history returns empty object
    transport.register("back", { ok: true, result: {} });

    const { result } = await gc.back();
    expect(transport.received[0]!.op).toBe("back");
    expect(result.url).toBeUndefined();
  });

  it("forward sends forward op", async () => {
    const { gc, transport } = makeClient();
    transport.register("forward", { ok: true, result: { url: "https://example.com", title: "Example Domain" } });

    const { result } = await gc.forward();
    expect(transport.received[0]!.op).toBe("forward");
    expect(result.url).toBe("https://example.com");
  });
});

// ---------------------------------------------------------------------------
// close / asyncDispose
// ---------------------------------------------------------------------------

describe("close", () => {
  it("sends close op and marks session as closed", async () => {
    const { gc, transport } = makeClient();
    // close emits no result
    transport.register("close", { ok: true });

    await gc.close();
    expect(transport.received[0]!.op).toBe("close");

    // After close, subsequent ops should throw
    await expect(gc.navigate("https://example.com")).rejects.toThrow("closed");
  });

  it("[Symbol.asyncDispose] closes the session", async () => {
    const transport = new FakeTransport();
    transport.register("close", { ok: true });
    {
      await using gc = new Ghostchrome({ transport });
      transport.register("navigate", {
        ok: true,
        result: { status: 200, url: "https://example.com", title: "Test", time_ms: 10 },
      });
      await gc.navigate("https://example.com");
    }
    // close op should have been sent
    const ops = transport.received.map((r) => r.op);
    expect(ops).toContain("close");
  });
});

// ---------------------------------------------------------------------------
// omitted-result ops normalise to {}
// ---------------------------------------------------------------------------

describe("omitted result normalisation", () => {
  it("click resolves with {} when binary omits result", async () => {
    const { gc, transport } = makeClient();
    // No result property — exactly what the binary emits for no-result ops
    transport.register("click", { ok: true });

    const { result } = await gc.click("@1");
    expect(result).toEqual({});
  });

  it("wait resolves with {} when binary omits result", async () => {
    const { gc, transport } = makeClient();
    transport.register("wait", { ok: true });

    const { result } = await gc.wait({ ms: 10 });
    expect(result).toEqual({});
  });
});

// ---------------------------------------------------------------------------
// Error handling
// ---------------------------------------------------------------------------

describe("error handling", () => {
  it("throws GhostchromeError when ok=false", async () => {
    const { gc, transport } = makeClient();
    transport.register("click", { ok: false, error: "stale ref @3" });

    await expect(gc.click("@3")).rejects.toThrow("stale ref @3");
  });
});

// ---------------------------------------------------------------------------
// Contract coverage — every JSONL op must have a method
// ---------------------------------------------------------------------------

describe("contract coverage", () => {
  it("every JSONL-surface op in contracts/commands.json has a client method", async () => {
    // Read the contract via Bun's file API
    // tests/ → typescript/ → sdk/ → repo-root/contracts/commands.json
    const contractPath = new URL("../../../contracts/commands.json", import.meta.url);
    const commands: Array<{ name: string; surfaces: string[] }> = await Bun.file(contractPath).json();

    const jsonlOps = commands
      .filter((c) => c.surfaces.includes("jsonl"))
      .map((c) => c.name);

    // Map JSONL snake_case op names → camelCase client method names
    const opToMethod: Record<string, string> = {
      init: "init",
      navigate: "navigate",
      back: "back",
      forward: "forward",
      reload: "reload",
      extract: "extract",
      click: "click",
      dblclick: "dblclick",
      check: "check",
      uncheck: "uncheck",
      hover: "hover",
      type: "type",
      press: "press",
      select: "select",
      fill: "fill",
      scroll_by: "scrollBy",
      scroll_to: "scrollTo",
      eval: "eval",
      screenshot: "screenshot",
      wait: "wait",
      errors: "errors",
      url: "url",
      close: "close",
    };

    const transport = new FakeTransport();
    const gc = new Ghostchrome({ transport });

    for (const op of jsonlOps) {
      const method = opToMethod[op];
      expect(method, `No mapping for JSONL op "${op}"`).toBeDefined();
      expect(
        typeof (gc as unknown as Record<string, unknown>)[method!],
        `Ghostchrome is missing method "${method}" for op "${op}"`
      ).toBe("function");
    }
  });
});
