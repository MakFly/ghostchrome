# @ghostchrome/sdk

TypeScript SDK for [ghostchrome](https://github.com/ghostchrome/ghostchrome) — typed async client for the JSONL agent loop.

All result shapes are measured from the live binary, never guessed.

## Install

```sh
bun add @ghostchrome/sdk
```

## Usage

```ts
import { createGhostchrome } from "@ghostchrome/sdk";

// Connect to an already-running Chrome (preferred — no startup cost)
const gc = createGhostchrome({ flags: ["--connect=auto"] });

// Navigate to a page
const { result: nav } = await gc.navigate("https://example.com");
console.log(nav.status, nav.url, nav.title, nav.time_ms);
// 200 "https://example.com/" "Example Domain" 20

// Extract accessibility tree (with refs)
const { result: snap } = await gc.extract({ level: "skeleton" });
console.log(snap.stats.interactive_count);
// 1
const linkRef = Object.entries(snap.refs).find(
  ([, n]) => n.name?.toLowerCase().includes("learn more")
)?.[0];

if (linkRef) {
  await gc.click(linkRef);
}

// Fill a form
const { result: filled } = await gc.fill({ "@3": "user@example.com", "@4": "s3cr3t" });
console.log(filled.filled); // number of fields typed

// Check current URL
const { result: urlResult } = await gc.url();
console.log("Now at:", urlResult.url, urlResult.title);

// Check console / network errors (returns a top-level array)
const { result: errs } = await gc.errors();
console.log(errs.length, errs[0]?.message);

// Scroll and get final position
const { result: scrolled } = await gc.scrollBy(300);
console.log("scrollY after:", scrolled.y);

// Screenshot — returns base64 + mime type
const { result: shot } = await gc.screenshot({ quality: 80 });
console.log(shot.mime, shot.base64.length);

// Always close cleanly
await gc.close();
```

### `using` (Symbol.asyncDispose)

```ts
{
  await using gc = createGhostchrome({ flags: ["--connect=auto"] });
  await gc.navigate("https://example.com");
  // close() is called automatically at block exit
}
```

## Transport injection (testing)

```ts
import { Ghostchrome } from "@ghostchrome/sdk";
import type { Transport } from "@ghostchrome/sdk";

class MyFakeTransport implements Transport { /* ... */ }

const gc = new Ghostchrome({ transport: new MyFakeTransport() });
```

## API

| Method | Description | Result type |
|---|---|---|
| `init()` | Open the browser session | `{}` |
| `navigate(url, opts?)` | Load a URL | `{ url, title, status, time_ms }` |
| `back()` | Navigate back | `{ url?, title? }` |
| `forward()` | Navigate forward | `{ url?, title? }` |
| `extract(opts?)` | Accessibility tree with @refs | `{ nodes, refs, stats }` |
| `click(ref)` | Click element by @ref | `{}` |
| `hover(ref)` | Hover over element | `{}` |
| `type(ref, text, opts?)` | Type text into input | `{}` |
| `press(key, opts?)` | Press a key | `{}` |
| `select(ref, values)` | Select option(s) | `{}` |
| `fill(fields)` | Fill form fields | `{ filled: number }` |
| `scrollBy(dy)` | Scroll viewport | `{ y: number }` |
| `scrollTo(opts)` | Scroll to position/bottom | `{ y: number }` |
| `eval(expr, opts?)` | Evaluate JS on page | `{ value: string }` |
| `screenshot(opts?)` | Capture image | `{ base64: string, mime: string }` |
| `wait(opts?)` | Wait for selector or delay | `{}` |
| `errors()` | Console + network errors | `ErrorEntry[]` |
| `url()` | Current URL + title | `{ url: string, title: string }` |
| `close()` | Close session | `{}` |

### ExtractResult.stats fields

```ts
stats: {
  total_nodes: number;       // total nodes in the a11y tree
  filtered_nodes: number;    // nodes after filtering
  interactive_count: number; // number of interactive refs assigned
}
```

### ErrorEntry shape

```ts
interface ErrorEntry {
  type: string;      // "console" | "network"
  level: string;     // "error" | "warning" | etc.
  message: string;
  source: string;
  status?: number;   // HTTP status (network errors only)
  method?: string;   // HTTP method (network errors only)
  time_ms: number;
}
```
