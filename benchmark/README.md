# Benchmark Harness

This folder contains a reproducible benchmark harness to compare browser stacks for LLM agents:

- `ghostchrome`
- `Playwright`

and agent CLIs:

- `Codex`
- `Claude Code`

The benchmark is task-based. It is designed to answer the only comparison that matters:

`tokens per successful task`

instead of only `tokens per snapshot`.

## What To Measure

For each run, record:

- `success`
- `duration_ms`
- `tool_calls`
- `input_tokens` and `output_tokens`

If exact token usage is unavailable, record:

- `input_chars`
- `output_chars`

The harness will estimate tokens as `ceil(chars / 4)` and mark those runs as estimated.

## Suite Format

Use one JSON file containing:

- suite metadata
- task definitions
- all runs

See [sample-suite.json](/Users/kev/Documents/lab/sandbox/ghostchrome/benchmark/sample-suite.json).

Each run should represent one attempt of one task with one pairing:

- `agent`: `codex` or `claude-code`
- `browser`: `ghostchrome` or `playwright`

Example run:

```json
{
  "task_id": "find-price",
  "agent": "codex",
  "browser": "ghostchrome",
  "success": true,
  "duration_ms": 3200,
  "tool_calls": 4,
  "input_tokens": 1300,
  "output_tokens": 250
}
```

## Recommended Protocol

Use the same machine, network, and target URLs for every run.

Run each task at least 5 times for each pairing:

- `codex + ghostchrome`
- `codex + playwright`
- `claude-code + ghostchrome`
- `claude-code + playwright`

Keep the task prompts identical across pairings.

Recommended task groups:

1. Inspection
2. Ecommerce navigation
3. Interaction

Recommended tasks:

1. Open the home page and list the main interactive elements.
2. Detect console and network errors after load.
3. Find a product and return price + URL.
4. Dismiss the cookie banner and re-extract the useful DOM.
5. Add one product to cart and report the final counter.

## Run The Comparator

```bash
go run ./benchmark/cmd/benchcmp --input benchmark/sample-suite.json
```

Write Markdown and JSON outputs:

```bash
go run ./benchmark/cmd/benchcmp \
  --input benchmark/my-suite.json \
  --markdown-out benchmark/latest-report.md \
  --json-out benchmark/latest-report.json
```

## Score Model

The default composite score is:

- `50%` success rate
- `25%` tokens per success
- `15%` median duration
- `10%` median tool calls

The report gives you:

- overall scoreboard
- winner by agent
- per-task comparison

## How To Collect Tokens

Preferred order:

1. Exact usage reported by the agent runtime or API.
2. Exact usage exported from CLI logs or traces.
3. Character-based estimate if exact usage is unavailable.

Do not mix measurement methods across pairings unless you have to. If one agent only gives estimates, record that in the run notes and keep the comparison honest.
