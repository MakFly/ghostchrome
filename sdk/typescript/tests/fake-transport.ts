/**
 * fake-transport.ts — In-memory FakeTransport for hermetic tests.
 *
 * Register canned responses keyed by op name; the FakeTransport
 * returns them when send() is called, correlating by the real request `id`.
 *
 * When no `result` is provided in the canned response (simulating ops
 * where the binary omits the field entirely), `result` is left undefined
 * in the OpResponse — the client normalises it to `{}`.
 */

import type { Transport } from "../src/transport.js";
import { GhostchromeError } from "../src/transport.js";
import type { OpRequest, OpResponse } from "../src/types.js";

export interface CannedResponse {
  ok: boolean;
  result?: unknown;
  observation?: unknown;
  error?: string;
}

export class FakeTransport implements Transport {
  /** Registered per-op canned responses. */
  private readonly canned = new Map<string, CannedResponse>();

  /** All requests received (for assertion). */
  readonly received: Array<OpRequest> = [];

  private isDisposed = false;

  /** Register a canned response for a given op name. */
  register(op: string, response: CannedResponse): this {
    this.canned.set(op, response);
    return this;
  }

  async send<R>(req: OpRequest): Promise<OpResponse<R>> {
    if (this.isDisposed) throw new Error("FakeTransport: disposed");
    this.received.push(req);

    const canned = this.canned.get(req.op);
    if (!canned) {
      // Default: succeed with empty result
      return { id: req.id, ok: true, result: {} as R };
    }

    const response: OpResponse<R> = {
      id: req.id,
      ok: canned.ok,
    };

    // Only attach result if the canned response provides it.
    // This mirrors the binary which omits the field for no-result ops
    // (wait and close; interaction ops now return snapshot diffs).
    if (canned.result !== undefined) {
      response.result = canned.result as R;
    }

    if (canned.observation !== undefined) {
      response.observation = canned.observation as never;
    }
    if (canned.error !== undefined) {
      response.error = canned.error;
    }

    if (!canned.ok) {
      throw new GhostchromeError(canned.error ?? "op failed", response);
    }

    return response;
  }

  async dispose(): Promise<void> {
    this.isDisposed = true;
  }
}
