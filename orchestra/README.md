# Find Food — Orchestra

Conversational agent that finds specific, orderable menu items (not just restaurants) matching a user's dietary restrictions near a location. Built on the [Mastra](https://mastra.ai) TypeScript agent framework, accessible from both a CLI and an HTTP/streaming API.

## Tech stack

| Concern | Choice |
|---|---|
| Agent framework | **Mastra** (`@mastra/core`) — agents, tools, agent loop |
| Model provider | **OpenRouter** via `@openrouter/ai-sdk-provider` (default `anthropic/claude-sonnet-4`) |
| Web search | **Exa**, accessed through its hosted **MCP server** (`@mastra/mcp`) |
| Memory | **Mastra memory** + **libsql** (`@mastra/memory`, `@mastra/libsql`) — conversation history + persistent per-user profile |
| Schemas / validation | **Zod** |
| Runtime | **Node ≥ 22.13** (see `.nvmrc`), TypeScript via `tsx` |

## Architecture

```txt
find-food (orchestrator agent)
  ├── memory ........... conversation history + persistent user profile (working memory)
  ├── Exa search tool .. discover candidate restaurants
  └── researchRestaurant tool
        └── research-restaurant (bounded agent)
              └── Exa search + fetch .. read each restaurant's menu/sources
              └── structured output ... validated ResearchResult (Zod)
```

The orchestrator chats with the user, asks for any missing inputs (food, location, dietary restrictions), runs Exa discovery, then delegates each candidate to a bounded `research-restaurant` agent that returns source-backed, structured menu items. Tool research is decoupled from structuring (a separate JSON pass) for reliable structured output across models.

## Project layout

```txt
src/
  cli.ts                       CLI adapter (readline -> agent stream -> stdout)
  server.ts                    HTTP adapter (POST /api/chat -> SSE stream)
  mastra/
    index.ts                   Mastra instance (registers agents + storage)
    run.ts                     runFindFoodTurn(): shared, transport-agnostic entry
    events.ts                  fullStream -> UI progress events (text/reasoning/tool)
    model.ts                   OpenRouter model wiring
    schemas.ts                 Zod schemas (research input/result, menu items)
    prompts.ts                 Agent instructions
    env.ts                     .env loading + helpers
    agents/
      find-food.ts             orchestrator agent
      research-restaurant.ts   bounded research agent
    tools/
      research-restaurant.ts   tool wrapping the research agent (structured output)
    mcp/
      exa.ts                   Exa MCP client + tool selection
    memory/
      index.ts                 Memory config (conversation + user profile)
```

## Setup

```bash
nvm use            # Node 22 (per .nvmrc); Mastra requires >= 22.13
npm install
```

Environment (read from `orchestra/.env` or the repo-root `.env`):

```bash
EXA_API_KEY=...            # required — Exa MCP search
OPENROUTER_API_KEY=...     # required — model provider

# optional
LLM_MODEL=anthropic/claude-sonnet-4
RESEARCH_MODEL=anthropic/claude-sonnet-4
PORT=3000
HOST=127.0.0.1
```

## Run

### CLI

```bash
npm run cli
```

```txt
you > gluten-free fish tacos in the Mission District, San Francisco
```

While it works you'll see live progress: `· searching the web: "…"`, `· researching <restaurant>`, then the streamed results. Commands: `reset` (new conversation), `quit`/`exit`. Run it in a real terminal so the dim status lines render.

### Web (HTTP + SSE)

```bash
npm run serve
```

```bash
# health
curl http://127.0.0.1:3000/health

# chat (keep the JSON on one line; a raw newline inside the string is invalid JSON)
curl -N -X POST http://127.0.0.1:3000/api/chat \
  -H 'Content-Type: application/json' \
  -d '{"message":"gluten-free fish tacos in the Mission District, San Francisco"}'
```

The response is an SSE stream of events:

```txt
{"type":"thread","threadId":"...","resourceId":"web-user"}
{"type":"tool","status":"start","label":"searching the web: ..."}
{"type":"tool","status":"start","label":"researching Loló"}
{"type":"delta","text":"..."}
{"type":"done"}
```

To continue a conversation, send the `threadId` from the first event back in the next request body. For multi-user setups, pass a stable user id as `resourceId` (the persistent profile is keyed by it).

### Mastra dev playground

```bash
npm run dev    # mastra dev — local playground UI + auto-exposed agent endpoints
```

## Memory

- **Conversation history** is kept per thread (last messages).
- **User profile** is persistent working memory scoped to the user (`resourceId`): dietary restrictions, allergies, usual location, and food likes/dislikes. The agent reads it to avoid re-asking and updates it when you state a durable fact ("I'm gluten-free, remember that"). Transient context (one-off cravings/locations) is not persisted.

Memory is stored in libsql files under `.mastra/` (gitignored).

## Observability & metrics

> **TODO (unresolved):** Metrics are still not showing up in Studio in the local setup, despite the libsql + DuckDB composite store and the wiring below. A test run via the API did persist traces and metrics (`/api/observability/metrics` returned data), so the remaining issue is likely Studio-side display (time range / refresh) or a CLI flush/lock interaction. Needs more debugging — paused for now.

Agent runs emit traces and metrics (duration, token usage, cost) automatically. Traces go to the libsql store; metrics require an OLAP store, so the observability domain is routed to **DuckDB** (`@mastra/duckdb`) via a composite store. Both DBs live under `.mastra/` (gitignored), so the CLI, the HTTP server, and `mastra dev` all read/write the same data.

View metrics and traces in the Studio dashboard:

```bash
npm run dev    # then open http://localhost:4111 -> Observability / Metrics
```

Metrics flush on a ~5s batch timer, so wait a few seconds after a run completes, and make sure the dashboard's time range includes "now".

### Requirements

- **Node 22+** is required (the `mastra` CLI uses APIs unavailable in Node 18). Run `nvm use` first.
- **One process at a time:** DuckDB allows only a single read-write process per file. Do **not** run the CLI/server while `mastra dev` is up (and vice versa) — the second process fails with a DuckDB lock error.

### Generating metrics from the CLI

`mastra dev` already runs the agents, so the easiest path is to chat in the Studio playground. To drive runs from the CLI instead:

1. Stop `mastra dev` (Ctrl-C) to release the DuckDB lock.
2. Run the CLI under Node 22:
   ```bash
   nvm use
   npm run cli
   ```
3. Chat a few turns. After the last response, **wait ~5s before typing `quit`** — the CLI exits without forcing a flush, so quitting immediately can drop the final turn's metrics.
4. Restart `npm run dev` to view the new metrics in Studio.

You can also fire a run over HTTP while `mastra dev` is up (it goes through the same server):

```bash
curl -s -X POST http://localhost:4111/api/agents/findFood/generate \
  -H 'Content-Type: application/json' \
  -d '{"messages":["Say hello in 3 words"]}'
# then inspect:
curl -s "http://localhost:4111/api/observability/metrics?perPage=10"
```

## Scripts

```bash
npm run cli         # interactive CLI
npm run serve       # HTTP + SSE server
npm run dev         # Mastra dev playground
npm run typecheck   # tsc --noEmit
npm run build       # tsc
```

## Notes

- Search uses Exa's hosted MCP server (`https://mcp.exa.ai/mcp`), not a direct REST client.
- Observability (traces + metrics) is built in via Mastra's storage exporter; metrics are persisted to DuckDB (see [Observability & metrics](#observability--metrics)). Mem0 is intentionally deferred; the current build relies on Mastra's built-in memory.
