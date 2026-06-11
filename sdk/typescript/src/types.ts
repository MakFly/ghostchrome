/**
 * types.ts — ghostchrome JSONL wire protocol types.
 * Shapes measured from the live binary — never guessed.
 */

// ---------------------------------------------------------------------------
// Wire envelope
// ---------------------------------------------------------------------------

/** A request sent on stdin to the ghostchrome agent. */
export interface OpRequest<A = Record<string, unknown>> {
  id: string;
  op: OpName;
  args?: A;
}

/**
 * A response received on stdout from the ghostchrome agent.
 *
 * - `result` is OMITTED (undefined) for ops that return nothing
 *   (init, wait, close, click, hover, type, press, select).
 * - `error` is present when `ok=false`.
 * - `events` is present only when the agent was started with `--observe`.
 * - `observation` is present whenever a page is active.
 */
export interface OpResponse<R = unknown> {
  id: string;
  ok: boolean;
  result?: R;
  error?: string;
  /** Raw CDP events captured during the op (only when --observe is active). */
  events?: unknown[];
  observation?: Observation;
}

// ---------------------------------------------------------------------------
// Observation packet
// ---------------------------------------------------------------------------

/**
 * Observation emitted with every response when a page is active.
 * Empty fields are omitted by the agent.
 */
export interface Observation {
  url?: string;
  console_errors?: unknown[];
  network_failed?: unknown[];
  a11y_diff?: string;
  dialog?: string;
  captcha_hint?: string;
}

// ---------------------------------------------------------------------------
// Op names (JSONL-surface only)
// ---------------------------------------------------------------------------

export type OpName =
  | "init"
  | "navigate"
  | "back"
  | "forward"
  | "extract"
  | "click"
  | "hover"
  | "type"
  | "press"
  | "select"
  | "fill"
  | "scroll_by"
  | "scroll_to"
  | "eval"
  | "screenshot"
  | "wait"
  | "errors"
  | "url"
  | "close";

// ---------------------------------------------------------------------------
// Op args — one interface per op
// ---------------------------------------------------------------------------

/** init: no args */
export type InitArgs = Record<string, never>;

/** navigate */
export interface NavigateArgs {
  url: string;
  /** load | stable | idle | none | domcontentloaded */
  wait?: string;
}

/** back / forward: no args */
export type BackArgs = Record<string, never>;
export type ForwardArgs = Record<string, never>;

/** extract */
export interface ExtractArgs {
  /** skeleton | content | full (default: content) */
  level?: "skeleton" | "content" | "full";
  /** Optional CSS selector to scope extraction */
  selector?: string;
}

/** click */
export interface ClickArgs {
  /** @ref of the element, e.g. @5 */
  ref: string;
}

/** hover */
export interface HoverArgs {
  /** @ref of the element */
  ref: string;
}

/** dblclick */
export interface DblClickArgs {
  /** @ref of the element, e.g. @5 */
  ref: string;
}

/** check / uncheck */
export interface CheckArgs {
  /** @ref of the checkbox/radio */
  ref: string;
}

/** type */
export interface TypeArgs {
  /** @ref of the input element */
  ref: string;
  /** Text to type (field is cleared first) */
  text: string;
  /** If true, clears the field before typing (default behaviour is already clear-first) */
  clear?: boolean;
  /** If true, press Enter after typing (submit the form) */
  submit?: boolean;
}

/** press */
export interface PressArgs {
  /** Key name, e.g. Enter, Escape, ArrowDown */
  key: string;
  /** Optional @ref to focus before pressing */
  ref?: string;
}

/** select */
export interface SelectArgs {
  /** @ref of the <select> element */
  ref: string;
  /** Option values to select */
  values: string[];
}

/** fill — map of @ref → value strings */
export interface FillArgs {
  fields: Record<string, string>;
}

/** scroll_by */
export interface ScrollByArgs {
  /** Pixels to scroll (signed, positive = down) */
  dy: number;
}

/** scroll_to */
export interface ScrollToArgs {
  /** Absolute Y coordinate */
  y?: number;
  /** If true, scroll to the page bottom */
  bottom?: boolean;
}

/** eval */
export interface EvalArgs {
  /** JS expression */
  expr: string;
  /** Optional @ref to bind as `this` */
  ref?: string;
}

/** screenshot */
export interface ScreenshotArgs {
  /** Capture full scrollable page (default: false) */
  full_page?: boolean;
  /** Capture only this element by @ref */
  ref?: string;
  /** JPEG quality 1-100 */
  quality?: number;
}

/** wait */
export interface WaitArgs {
  /** CSS selector to wait for */
  selector?: string;
  /** Fixed delay in milliseconds */
  ms?: number;
}

/** errors / url / close: no args */
export type ErrorsArgs = Record<string, never>;
export type UrlArgs = Record<string, never>;
export type CloseArgs = Record<string, never>;

// ---------------------------------------------------------------------------
// Op results — shapes measured from the live binary
// ---------------------------------------------------------------------------

/**
 * init — binary omits `result`; client resolves with `{}`.
 */
export type InitResult = Record<string, never>;

/**
 * navigate result.
 * Measured: { url:string, title:string, status:number, time_ms:number }
 */
export interface NavigateResult {
  url: string;
  title: string;
  status: number;
  /** Navigation duration in milliseconds. */
  time_ms: number;
}

/**
 * back / forward result.
 * Measured: { url?:string, title?:string }  (may be {} when no history entry exists)
 */
export interface BackResult {
  url?: string;
  title?: string;
}
export type ForwardResult = BackResult;

/** reload result: { url?:string, title?:string } */
export type ReloadResult = BackResult;

/** A single accessibility node with an optional @ref. */
export interface A11yNode {
  role?: string;
  name?: string;
  ref?: string;
  text?: string;
  children?: A11yNode[];
  [key: string]: unknown;
}

/** A ref entry in the refs map (keyed by "@N"). */
export interface RefEntry {
  role?: string;
  name?: string;
  text?: string;
  [key: string]: unknown;
}

/**
 * extract result.
 * Measured: { nodes:A11yNode[], refs:Record<string,RefEntry>,
 *             stats:{ total_nodes:number, filtered_nodes:number, interactive_count:number } }
 */
export interface ExtractResult {
  nodes: A11yNode[];
  refs: Record<string, RefEntry>;
  stats: {
    total_nodes: number;
    filtered_nodes: number;
    interactive_count: number;
  };
}

/**
 * Ops that emit no result (init, wait, close, click, hover, type, press, select).
 * The binary omits `result` entirely; the client resolves with `{}`.
 */
export type EmptyResult = Record<string, never>;

/**
 * fill result.
 * Measured: { filled:number }  (count of fields successfully typed)
 */
export interface FillResult {
  filled: number;
}

/**
 * scroll_by result.
 * Measured: { y:number }  (final scrollY after the scroll)
 */
export interface ScrollByResult {
  y: number;
}

/**
 * scroll_to result.
 * Measured: { y:number }
 */
export interface ScrollToResult {
  y: number;
}

/**
 * eval result.
 * Measured: { value:string }  (binary always JSON-stringifies the JS return value)
 */
export interface EvalResult {
  value: string;
}

/**
 * screenshot result.
 * Measured: { base64:string, mime:string }
 */
export interface ScreenshotResult {
  base64: string;
  mime: string;
}

/** wait — binary omits `result`; client resolves with `{}`. */
export type WaitResult = Record<string, never>;

/**
 * errors result.
 * Measured: top-level ARRAY (NOT wrapped in {errors:[]}).
 * Each element: { type:string, level:string, message:string, source:string,
 *                 status?:number, method?:string, time_ms:number }
 */
export interface ErrorEntry {
  type: string;
  level: string;
  message: string;
  source: string;
  status?: number;
  method?: string;
  time_ms: number;
}

/** errors() resolves to an array of ErrorEntry. */
export type ErrorsResult = ErrorEntry[];

/**
 * url result.
 * Measured: { url:string, title:string }
 */
export interface UrlResult {
  url: string;
  title: string;
}

/** close — binary omits `result`; client resolves with `{}`. */
export type CloseResult = Record<string, never>;

// ---------------------------------------------------------------------------
// Typed response helpers
// ---------------------------------------------------------------------------

export type NavigateResponse = OpResponse<NavigateResult>;
export type ExtractResponse = OpResponse<ExtractResult>;
export type EvalResponse = OpResponse<EvalResult>;
export type ScreenshotResponse = OpResponse<ScreenshotResult>;
export type ErrorsResponse = OpResponse<ErrorsResult>;
export type UrlResponse = OpResponse<UrlResult>;
