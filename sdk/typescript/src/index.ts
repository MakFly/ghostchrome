/**
 * index.ts — public API surface of @ghostchrome/sdk
 */

// Client
export { Ghostchrome, createGhostchrome } from "./client.js";
export type { GhostchromeOptions, OpOutcome } from "./client.js";

// Transport
export { SubprocessTransport, GhostchromeError } from "./transport.js";
export type { Transport, SubprocessTransportOptions } from "./transport.js";

// Types (wire protocol)
export type {
  OpRequest,
  OpResponse,
  OpName,
  Observation,
  // args
  InitArgs,
  NavigateArgs,
  BackArgs,
  ForwardArgs,
  ExtractArgs,
  ClickArgs,
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
  ErrorsArgs,
  UrlArgs,
  CloseArgs,
  TabsArgs,
  DialogArgs,
  // results
  InitResult,
  SnapshotDiff,
  MutationResult,
  DiffNode,
  DiffEntry,
  DiffStats,
  NavigateResult,
  BackResult,
  ForwardResult,
  ExtractResult,
  A11yNode,
  RefEntry,
  EmptyResult,
  FillResult,
  ScrollByResult,
  ScrollToResult,
  EvalResult,
  ScreenshotResult,
  WaitResult,
  ErrorEntry,
  ErrorsResult,
  UrlResult,
  CloseResult,
  TabsResult,
  DialogResult,
  TabInfo,
  TabsActionResult,
  NavigateResponse,
  ExtractResponse,
  EvalResponse,
  ScreenshotResponse,
  ErrorsResponse,
  UrlResponse,
} from "./types.js";
