/**
 * client.ts — Ghostchrome typed async client.
 *
 * Wraps a Transport and exposes one async method per JSONL op.
 * Generates unique op IDs, sends the request, and resolves with
 * the typed result + observation.
 *
 * Interaction ops return a SnapshotDiff. wait and close may omit `result` and
 * are normalized to an empty object.
 *
 * On ok=false the transport throws GhostchromeError carrying
 * { id, op, error } before this layer is reached.
 */

import type { Transport, SubprocessTransportOptions } from "./transport.js";
import { SubprocessTransport } from "./transport.js";
import type {
  OpResponse,
  Observation,
  // args
  NavigateArgs,
  ExtractArgs,
  ClickArgs,
  DblClickArgs,
  CheckArgs,
  HoverArgs,
  TypeArgs,
  PressArgs,
  SelectArgs,
  FillArgs,
  ScrollByArgs,
  ScrollToArgs,
  EvalArgs,
  ScreenshotArgs,
  WaitArgs,
  TabsArgs,
  DialogArgs,
  // results
  InitResult,
  NavigateResult,
  BackResult,
  ForwardResult,
  ReloadResult,
  ExtractResult,
  EmptyResult,
  FillResult,
  ScrollByResult,
  ScrollToResult,
  EvalResult,
  ScreenshotResult,
  ErrorsResult,
  UrlResult,
  CloseResult,
  TabsResult,
  DialogResult,
  MutationResult,
} from "./types.js";

// ---------------------------------------------------------------------------
// Result envelope returned to the caller
// ---------------------------------------------------------------------------

/** Combines the op result and the observation in one object. */
export interface OpOutcome<R> {
  result: R;
  observation?: Observation | undefined;
}

// ---------------------------------------------------------------------------
// Ghostchrome client options
// ---------------------------------------------------------------------------

export interface GhostchromeOptions {
  /**
   * Provide an existing Transport (useful for testing with a FakeTransport).
   * When omitted, a SubprocessTransport is created automatically.
   */
  transport?: Transport;
  /** Options forwarded to SubprocessTransport when no transport is provided. */
  subprocess?: SubprocessTransportOptions;
}

// ---------------------------------------------------------------------------
// Ghostchrome class
// ---------------------------------------------------------------------------

let idCounter = 0;

function nextId(): string {
  idCounter += 1;
  return `gc${idCounter}`;
}

export class Ghostchrome {
  private readonly transport: Transport;
  private closed = false;

  constructor(options: GhostchromeOptions = {}) {
    this.transport = options.transport ?? new SubprocessTransport(options.subprocess);
  }

  // -------------------------------------------------------------------------
  // Internal helpers
  // -------------------------------------------------------------------------

  /**
   * Send an op and return the typed outcome.
   * When the binary omits `result` (undefined), falls back to `{}` cast as R.
   */
  private async op<A, R>(
    opName: string,
    args?: A
  ): Promise<OpOutcome<R>> {
    if (this.closed) throw new Error("Ghostchrome: session is closed");
    const id = nextId();
    const response = (await this.transport.send<R>({
      id,
      op: opName as never,
      args: args as Record<string, unknown>,
    })) as OpResponse<R>;

    // result may be omitted for no-result ops — normalise to {} so callers
    // always get a defined value without crashing on destructuring.
    const result: R = response.result !== undefined
      ? response.result
      : ({} as R);

    const outcome: OpOutcome<R> = { result };
    if (response.observation !== undefined) {
      outcome.observation = response.observation;
    }
    return outcome;
  }

  /** Shorthand for ops that reliably return no result. */
  private async emptyOp<A>(opName: string, args?: A): Promise<OpOutcome<EmptyResult>> {
    return this.op<A, EmptyResult>(opName, args);
  }

  // -------------------------------------------------------------------------
  // Session lifecycle
  // -------------------------------------------------------------------------

  /**
   * init — open the browser (no-op if already open).
   * Call this before the first op when using --connect=auto or a pre-existing session.
   */
  async init(): Promise<OpOutcome<InitResult>> {
    return this.op<Record<string, never>, InitResult>("init");
  }

  /**
   * close — send the close op, then dispose the transport.
   * After this call the instance cannot be reused.
   */
  async close(): Promise<OpOutcome<CloseResult>> {
    if (this.closed) return { result: {} };
    const outcome = await this.emptyOp<Record<string, never>>("close");
    this.closed = true;
    await this.transport.dispose();
    return outcome as OpOutcome<CloseResult>;
  }

  // -------------------------------------------------------------------------
  // Navigation
  // -------------------------------------------------------------------------

  /** navigate — load a URL in the current tab. */
  async navigate(url: string, opts?: Omit<NavigateArgs, "url">): Promise<OpOutcome<NavigateResult>> {
    return this.op<NavigateArgs, NavigateResult>("navigate", { url, ...opts });
  }

  /** back — navigate back in browser history. */
  async back(): Promise<OpOutcome<BackResult>> {
    return this.op<Record<string, never>, BackResult>("back");
  }

  /** forward — navigate forward in browser history. */
  async forward(): Promise<OpOutcome<ForwardResult>> {
    return this.op<Record<string, never>, ForwardResult>("forward");
  }

  /** reload — reload (refresh) the current page. */
  async reload(): Promise<OpOutcome<ReloadResult>> {
    return this.op<Record<string, never>, ReloadResult>("reload");
  }

  // -------------------------------------------------------------------------
  // Extraction
  // -------------------------------------------------------------------------

  /** extract — return a compact accessibility tree with @refs. */
  async extract(opts?: ExtractArgs): Promise<OpOutcome<ExtractResult>> {
    return this.op<ExtractArgs, ExtractResult>("extract", opts);
  }

  // -------------------------------------------------------------------------
  // Interaction
  // -------------------------------------------------------------------------

  /** click — click an element by its @ref. */
  async click(ref: string, opts?: Pick<ClickArgs, "snapshot" | "button">): Promise<OpOutcome<MutationResult>> {
    return this.op<ClickArgs, MutationResult>("click", { ref, ...opts });
  }

  /** dblclick — double-click an element by its @ref. */
  async dblclick(ref: string, opts?: Pick<DblClickArgs, "snapshot" | "button">): Promise<OpOutcome<MutationResult>> {
    return this.op<DblClickArgs, MutationResult>("dblclick", { ref, ...opts });
  }

  /** check — tick a checkbox/radio by @ref (idempotent). */
  async check(ref: string, opts?: Pick<CheckArgs, "snapshot">): Promise<OpOutcome<MutationResult>> {
    return this.op<CheckArgs, MutationResult>("check", { ref, ...opts });
  }

  /** uncheck — untick a checkbox by @ref (idempotent). */
  async uncheck(ref: string, opts?: Pick<CheckArgs, "snapshot">): Promise<OpOutcome<MutationResult>> {
    return this.op<CheckArgs, MutationResult>("uncheck", { ref, ...opts });
  }

  /** hover — hover over an element by @ref (reveals dropdowns, tooltips). */
  async hover(ref: string, opts?: Pick<HoverArgs, "snapshot">): Promise<OpOutcome<MutationResult>> {
    return this.op<HoverArgs, MutationResult>("hover", { ref, ...opts });
  }

  /**
   * type — type text into an input/textarea by @ref.
   * The field is cleared before typing by default.
   */
  async type(ref: string, text: string, opts?: Pick<TypeArgs, "clear" | "submit" | "snapshot">): Promise<OpOutcome<MutationResult>> {
    return this.op<TypeArgs, MutationResult>("type", { ref, text, ...opts });
  }

  /** press — press a keyboard key; optionally focus an element by @ref first. */
  async press(key: string, opts?: Pick<PressArgs, "ref" | "snapshot">): Promise<OpOutcome<MutationResult>> {
    return this.op<PressArgs, MutationResult>("press", { key, ...opts });
  }

  /** select — pick one or more options in a <select> element by @ref. */
  async select(ref: string, values: string[], opts?: Pick<SelectArgs, "snapshot">): Promise<OpOutcome<MutationResult>> {
    return this.op<SelectArgs, MutationResult>("select", { ref, values, ...opts });
  }

  /**
   * fill — fill multiple form fields in one call.
   * @param fields — map of @ref → value strings
   * Returns { filled: number } — count of fields successfully typed.
   */
  async fill(fields: Record<string, string>): Promise<OpOutcome<FillResult>> {
    return this.op<FillArgs, FillResult>("fill", { fields });
  }

  // -------------------------------------------------------------------------
  // Scrolling
  // -------------------------------------------------------------------------

  /**
   * scrollBy — scroll the viewport by dy pixels (positive = down).
   * Returns { y: number } — final scrollY position after the scroll.
   */
  async scrollBy(dy: number): Promise<OpOutcome<ScrollByResult>> {
    return this.op<ScrollByArgs, ScrollByResult>("scroll_by", { dy });
  }

  /**
   * scrollTo — scroll to an absolute Y position or to the page bottom.
   * Returns { y: number } — final scrollY position.
   */
  async scrollTo(opts: ScrollToArgs): Promise<OpOutcome<ScrollToResult>> {
    return this.op<ScrollToArgs, ScrollToResult>("scroll_to", opts);
  }

  // -------------------------------------------------------------------------
  // Evaluation
  // -------------------------------------------------------------------------

  /** eval — evaluate a JavaScript expression on the page. */
  async eval(expr: string, opts?: Pick<EvalArgs, "ref">): Promise<OpOutcome<EvalResult>> {
    return this.op<EvalArgs, EvalResult>("eval", { expr, ...opts });
  }

  // -------------------------------------------------------------------------
  // Screenshot
  // -------------------------------------------------------------------------

  /** screenshot — capture the current viewport (or element) as a PNG/JPEG image. */
  async screenshot(opts?: ScreenshotArgs): Promise<OpOutcome<ScreenshotResult>> {
    return this.op<ScreenshotArgs, ScreenshotResult>("screenshot", opts);
  }

  // -------------------------------------------------------------------------
  // Waiting
  // -------------------------------------------------------------------------

  /**
   * wait — wait for a CSS selector to appear or a fixed delay.
   * @param opts.selector — CSS selector to wait for
   * @param opts.ms — fixed delay in milliseconds
   */
  async wait(opts?: WaitArgs): Promise<OpOutcome<EmptyResult>> {
    return this.emptyOp<WaitArgs>("wait", opts);
  }

  // -------------------------------------------------------------------------
  // Inspection
  // -------------------------------------------------------------------------

  /**
   * errors — return console and network errors observed on the current page.
   * Resolves to a top-level array of ErrorEntry objects (not wrapped in an object).
   */
  async errors(): Promise<OpOutcome<ErrorsResult>> {
    return this.op<Record<string, never>, ErrorsResult>("errors");
  }

  /** url — return the current page URL and title. */
  async url(): Promise<OpOutcome<UrlResult>> {
    return this.op<Record<string, never>, UrlResult>("url");
  }

  /** tabs — list, switch, close, or open browser tabs. */
  async tabs(opts?: TabsArgs): Promise<OpOutcome<TabsResult>> {
    return this.op<TabsArgs, TabsResult>("tabs", opts);
  }

  /** dialog — set auto-accept/dismiss for JS alert/confirm/prompt. */
  async dialog(opts?: DialogArgs): Promise<OpOutcome<DialogResult>> {
    return this.op<DialogArgs, DialogResult>("dialog", opts);
  }

  // -------------------------------------------------------------------------
  // Symbol.asyncDispose (using-friendly)
  // -------------------------------------------------------------------------

  async [Symbol.asyncDispose](): Promise<void> {
    if (!this.closed) {
      await this.close();
    }
  }
}

// ---------------------------------------------------------------------------
// Factory helpers
// ---------------------------------------------------------------------------

/**
 * Create a Ghostchrome client backed by a subprocess transport.
 *
 * @example
 * ```ts
 * const gc = createGhostchrome({ flags: ["--connect=auto"] });
 * const { result: nav } = await gc.navigate("https://example.com");
 * console.log(nav.status, nav.title, nav.time_ms);
 * const { result: snap } = await gc.extract({ level: "skeleton" });
 * console.log(snap.stats.interactive_count);
 * const errs = (await gc.errors()).result; // ErrorEntry[]
 * await gc.close();
 * ```
 */
export function createGhostchrome(opts?: SubprocessTransportOptions): Ghostchrome {
  if (opts !== undefined) {
    return new Ghostchrome({ subprocess: opts });
  }
  return new Ghostchrome();
}
