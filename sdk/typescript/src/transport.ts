/**
 * transport.ts — Transport abstraction + SubprocessTransport implementation.
 *
 * SubprocessTransport spawns `ghostchrome agent` as a long-lived child process,
 * writes JSONL ops to its stdin, reads JSONL responses from stdout, and
 * correlates responses by `id` using a pending-promise map.
 */

import { spawn, type ChildProcess } from "node:child_process";
import type { OpRequest, OpResponse } from "./types.js";

// ---------------------------------------------------------------------------
// Transport interface
// ---------------------------------------------------------------------------

/** A transport sends op requests and resolves typed responses. */
export interface Transport {
  /**
   * Send an op and return its response.
   * Implementations must correlate responses by `id`.
   */
  send<R>(req: OpRequest): Promise<OpResponse<R>>;

  /** Dispose the transport (close the underlying channel). */
  dispose(): Promise<void>;
}

// ---------------------------------------------------------------------------
// SubprocessTransport options
// ---------------------------------------------------------------------------

export interface SubprocessTransportOptions {
  /** Binary to spawn. Default: "ghostchrome". */
  command?: string;
  /** Arguments after the binary. Default: ["agent"]. */
  args?: string[];
  /** Extra flags appended after args (e.g. ["--stealth", "--observe"]). */
  flags?: string[];
  /** Environment variables for the child process. Default: inherits process.env. */
  env?: NodeJS.ProcessEnv;
  /** Timeout in ms for each op. 0 = no timeout. Default: 30000 (matches the Python SDK). */
  opTimeoutMs?: number;
}

// ---------------------------------------------------------------------------
// SubprocessTransport
// ---------------------------------------------------------------------------

interface PendingRequest {
  resolve: (value: OpResponse<unknown>) => void;
  reject: (reason: unknown) => void;
  timer?: ReturnType<typeof setTimeout> | undefined;
}

export class SubprocessTransport implements Transport {
  private readonly command: string;
  private readonly spawnArgs: string[];
  private readonly env: NodeJS.ProcessEnv | undefined;
  private readonly opTimeoutMs: number;

  private proc: ChildProcess | null = null;
  private readonly pending = new Map<string, PendingRequest>();
  private lineBuffer = "";
  private disposed = false;

  constructor(options: SubprocessTransportOptions = {}) {
    this.command = options.command ?? "ghostchrome";
    const base = options.args ?? ["agent"];
    const flags = options.flags ?? [];
    this.spawnArgs = [...base, ...flags];
    this.env = options.env;
    this.opTimeoutMs = options.opTimeoutMs ?? 30_000;
  }

  // -------------------------------------------------------------------------
  // Lifecycle
  // -------------------------------------------------------------------------

  /** Start the child process. Called lazily on first send() if not called explicitly. */
  start(): void {
    if (this.proc !== null) return;
    if (this.disposed) throw new Error("SubprocessTransport: already disposed");

    this.proc = spawn(this.command, this.spawnArgs, {
      stdio: ["pipe", "pipe", "inherit"],
      env: this.env ?? process.env,
    });

    if (!this.proc.stdout) {
      throw new Error("SubprocessTransport: child process has no stdout");
    }
    if (!this.proc.stdin) {
      throw new Error("SubprocessTransport: child process has no stdin");
    }

    // Line-buffer stdout — handle partial chunks
    this.proc.stdout.setEncoding("utf8");
    this.proc.stdout.on("data", (chunk: string) => {
      this.lineBuffer += chunk;
      let newlineIdx: number;
      while ((newlineIdx = this.lineBuffer.indexOf("\n")) !== -1) {
        const line = this.lineBuffer.slice(0, newlineIdx).trim();
        this.lineBuffer = this.lineBuffer.slice(newlineIdx + 1);
        if (line.length > 0) {
          this.handleLine(line);
        }
      }
    });

    this.proc.on("error", (err) => {
      this.rejectAll(err);
    });

    this.proc.on("close", (code) => {
      const err = new Error(
        `SubprocessTransport: ghostchrome process exited with code ${code}`
      );
      this.rejectAll(err);
      this.proc = null;
    });
  }

  // -------------------------------------------------------------------------
  // Transport.send
  // -------------------------------------------------------------------------

  async send<R>(req: OpRequest): Promise<OpResponse<R>> {
    if (this.disposed) throw new Error("SubprocessTransport: already disposed");
    if (this.proc === null) this.start();

    const proc = this.proc!;
    if (!proc.stdin || proc.stdin.destroyed) {
      throw new Error("SubprocessTransport: stdin is closed");
    }

    return new Promise<OpResponse<R>>((resolve, reject) => {
      let timer: ReturnType<typeof setTimeout> | undefined;
      if (this.opTimeoutMs > 0) {
        timer = setTimeout(() => {
          this.pending.delete(req.id);
          reject(
            new Error(
              `SubprocessTransport: op "${req.op}" (id=${req.id}) timed out after ${this.opTimeoutMs}ms`
            )
          );
        }, this.opTimeoutMs);
      }

      this.pending.set(req.id, {
        resolve: resolve as (v: OpResponse<unknown>) => void,
        reject,
        timer,
      });

      const line = JSON.stringify(req) + "\n";
      proc.stdin!.write(line, "utf8", (err) => {
        if (err) {
          const pending = this.pending.get(req.id);
          if (pending) {
            this.pending.delete(req.id);
            if (pending.timer) clearTimeout(pending.timer);
            reject(err);
          }
        }
      });
    });
  }

  // -------------------------------------------------------------------------
  // Transport.dispose
  // -------------------------------------------------------------------------

  async dispose(): Promise<void> {
    if (this.disposed) return;
    this.disposed = true;

    if (this.proc && this.proc.stdin && !this.proc.stdin.destroyed) {
      this.proc.stdin.end();
    }

    await new Promise<void>((resolve) => {
      if (!this.proc) {
        resolve();
        return;
      }
      // Escalate: graceful close → SIGTERM at 2s → SIGKILL at 4s. A binary
      // that ignores SIGTERM must never leave a zombie agent behind.
      const term = setTimeout(() => this.proc?.kill("SIGTERM"), 2000);
      const kill = setTimeout(() => {
        this.proc?.kill("SIGKILL");
        resolve();
      }, 4000);
      this.proc.once("close", () => {
        clearTimeout(term);
        clearTimeout(kill);
        resolve();
      });
    });

    this.rejectAll(new Error("SubprocessTransport: disposed"));
    this.proc = null;
  }

  // -------------------------------------------------------------------------
  // Internal
  // -------------------------------------------------------------------------

  private handleLine(line: string): void {
    let response: OpResponse<unknown>;
    try {
      response = JSON.parse(line) as OpResponse<unknown>;
    } catch {
      // Non-JSON line from subprocess (e.g. debug output) — ignore
      return;
    }

    const { id } = response;
    const pending = this.pending.get(id);
    if (!pending) return; // Unsolicited or duplicate response

    this.pending.delete(id);
    if (pending.timer) clearTimeout(pending.timer);

    if (!response.ok) {
      pending.reject(
        new GhostchromeError(response.error ?? "op failed", response)
      );
    } else {
      pending.resolve(response);
    }
  }

  private rejectAll(err: Error): void {
    for (const [id, pending] of this.pending) {
      if (pending.timer) clearTimeout(pending.timer);
      pending.reject(err);
      this.pending.delete(id);
    }
  }
}

// ---------------------------------------------------------------------------
// Error type
// ---------------------------------------------------------------------------

/** Thrown when a ghostchrome op returns ok=false. */
export class GhostchromeError extends Error {
  constructor(
    message: string,
    public readonly response: OpResponse<unknown>
  ) {
    super(message);
    this.name = "GhostchromeError";
  }
}
