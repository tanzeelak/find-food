# Find Food — Orchestra

Conversational agent that finds specific, orderable menu items (not just restaurants) matching a user's dietary restrictions near a location. Built on the [Mastra](https://mastra.ai) TypeScript agent framework, accessible from a CLI, an HTTP/streaming API, and a React chat UI (Assistant UI, in the sibling [`app/`](../app) directory).

## Tech stack

| Concern | Choice |
|---|---|
| Agent framework | **Mastra** (`@mastra/core`) — agents, tools, agent loop |
| Model provider | **OpenRouter** via `@openrouter/ai-sdk-provider` (default `anthropic/claude-sonnet-4`) |
| Web search | **Exa**, accessed through its hosted **MCP server** (`@mastra/mcp`) |
| Chat API | **AI SDK-compatible route** via `@mastra/ai-sdk` `chatRoute()` — `POST /chat/:agentId` |
| Frontend | **Assistant UI** (`@assistant-ui/react`, `@assistant-ui/react-ai-sdk`) on **Next.js** — sibling [`app/`](../app) |
| Memory | **Mastra memory** + **libsql** (`@mastra/memory`, `@mastra/libsql`) — conversation history + persistent per-user profile |
| Storage | **libsql** (`@mastra/libsql`) for the default store, **DuckDB** (`@mastra/duckdb`) for the observability/metrics domain, wired together with a `MastraCompositeStore` |
| Observability | **Mastra observability** (`@mastra/observability`) — traces + metrics via `MastraStorageExporter` |
| Hosting (prod) | **Mastra platform** — deployed Studio project (`.mastra-project.json`) |
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
../app/                        Next.js + Assistant UI chat frontend (separate package)
src/
  cli.ts                       CLI adapter (readline -> agent stream -> stdout)
  server.ts                    HTTP adapter (POST /api/chat -> SSE stream)
  mastra/
    index.ts                   Mastra instance (agents + storage + chatRoute /chat/:agentId)
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

# optional — shared cloud store (libsql/Turso). The auth token is a per-database
# token (independent of `turso auth login`). When unset, the default store and
# memory fall back to local files under .mastra/ (see Storage & memory > Turso).
MASTRA_DB_URL=libsql://<db-name>-<org>.<region>.turso.io
MASTRA_DB_AUTH_TOKEN=...

# optional — Mastra platform (prod Studio). Set by `mastra` tooling; lets
# the deployed project authenticate against the platform.
MASTRA_PLATFORM_ACCESS_TOKEN=...
MASTRA_PLATFORM_PROJECT_ID=...

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

### Assistant UI frontend (Next.js)

A React chat UI lives in the sibling [`app/`](../app) directory (Next.js + Tailwind + [Assistant UI](https://www.assistant-ui.com/)). It talks to the agent through an AI SDK-compatible endpoint exposed by `chatRoute()` in `src/mastra/index.ts`:

```ts
import { chatRoute } from "@mastra/ai-sdk";
// ...
server: { apiRoutes: [ chatRoute({ path: "/chat/:agentId", sendReasoning: true }) ] }
```

This makes every registered agent available in AI SDK format; `findFood` is served at **`POST /chat/findFood`**. The route is served by `mastra dev` (and the deployed Mastra server) — **not** by the custom `npm run serve` HTTP server (`src/server.ts`), which is a separate hand-rolled transport.

Run both halves (two terminals):

```bash
# terminal 1 — agent server (this directory)
nvm use && npm run dev                     # mastra dev on http://localhost:4111

# terminal 2 — frontend (repo-root app/)
cd ../app && npm install && npm run dev     # Next.js on http://localhost:3000
```

The frontend points at `http://localhost:4111/chat/findFood` by default. Override the base URL with `NEXT_PUBLIC_MASTRA_URL` in `app/.env.local` — e.g. set it to `https://food-agent.server.mastra.cloud` to chat against prod. The wiring lives in `app/src/components/assistant.tsx` (`useChatRuntime` + `AssistantChatTransport`).

## Storage & memory

The Mastra instance uses a **`MastraCompositeStore`** (`src/mastra/index.ts`):

- **Default store → libsql.** Conversation history, user profile, and traces live here. If `MASTRA_DB_URL` (+ `MASTRA_DB_AUTH_TOKEN`) is set, it points at a shared cloud DB (Turso); otherwise it falls back to a local file at `.mastra/mastra.db`.
- **Observability domain → DuckDB.** Metrics need an OLAP store, so the observability domain is routed to a DuckDB file at `.mastra/find-food-observability.duckdb`. See [Observability & metrics](#observability--metrics).

Memory specifics:

- **Conversation history** is kept per thread (last messages).
- **User profile** is persistent working memory scoped to the user (`resourceId`): dietary restrictions, allergies, usual location, and food likes/dislikes. The agent reads it to avoid re-asking and updates it when you state a durable fact ("I'm gluten-free, remember that"). Transient context (one-off cravings/locations) is not persisted.

Memory has its own connection resolver (`src/mastra/memory/index.ts`): it uses `MEMORY_DB_URL` / `MEMORY_DB_AUTH_TOKEN` if set, otherwise **inherits `MASTRA_DB_URL`**, otherwise falls back to a local file `.mastra/find-food-memory.db`. So if `MASTRA_DB_URL` points at Turso, conversation history and the user profile are written to Turso too.

Local DB files live under `.mastra/` (gitignored).

### Turso (cloud libsql)

[Turso](https://turso.tech) is a hosted [libsql](https://github.com/tursodatabase/libsql) database — SQLite's storage engine with a remote, network-accessible server. The app treats it as a drop-in libsql store, so the only difference from a local SQLite file is the connection target.

**How the app connects.** Two env vars (in `orchestra/.env` or repo-root `.env`):

```bash
MASTRA_DB_URL=libsql://<db-name>-<org>.<region>.turso.io   # e.g. find-food-tanzeelak.aws-us-east-2.turso.io
MASTRA_DB_AUTH_TOKEN=<database-token>                       # JWT scoped to this DB
```

The auth token is a **per-database token** and is independent of the Turso CLI login — the running app authenticates with this token alone, so you do *not* need `turso auth login` for the app to work. When `MASTRA_DB_URL` is unset, everything falls back to local files under `.mastra/` and Turso is not involved.

**What lives in the DB.** Mastra creates `mastra_*` tables; the ones that matter here:

| Table | Contents |
|---|---|
| `mastra_threads` | one row per conversation |
| `mastra_messages` | individual chat messages |
| `mastra_resources` | per-user working memory (the food profile, keyed by `resourceId`) |
| `mastra_ai_spans` | observability **traces** |

Note: libsql/Turso stores **traces, not metrics** — metrics go to DuckDB (see [Observability & metrics](#observability--metrics)).

**Inspecting it.** Install the CLI and authenticate once (browser OAuth):

```bash
brew install tursodatabase/tap/turso   # or: curl -sSfL https://get.tur.so/install.sh | bash
turso auth login
turso db list                          # confirm the db name
```

Then open a SQL shell against it (the `db:shell` script targets the `find-food` DB):

```bash
npm run db:shell
# equivalently:
turso db shell find-food
```

**Sample queries.** Run these in `npm run db:shell`, or pass them inline as `turso db shell find-food "<sql>"`:

```sql
-- list all tables
SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name;

-- saved user profiles (working memory), one row per resourceId
SELECT id, workingMemory FROM mastra_resources;

-- recent conversations
SELECT id, resourceId, title, createdAt
FROM mastra_threads
ORDER BY createdAt DESC
LIMIT 10;

-- all threads for a specific user
SELECT id, title, createdAt
FROM mastra_threads
WHERE resourceId = 'cli-user'
ORDER BY createdAt DESC;

-- message count per thread
SELECT thread_id, COUNT(*) AS messages
FROM mastra_messages
GROUP BY thread_id
ORDER BY messages DESC;

-- full transcript of one thread (oldest first)
SELECT role, createdAt, content
FROM mastra_messages
WHERE thread_id = '<thread-id>'
ORDER BY createdAt;

-- recent observability traces / agent runs
SELECT name, spanType, startedAt, endedAt
FROM mastra_ai_spans
ORDER BY startedAt DESC
LIMIT 20;

-- top-level (root) spans only
SELECT name, startedAt, endedAt
FROM mastra_ai_spans
WHERE parentSpanId IS NULL
ORDER BY startedAt DESC
LIMIT 10;

-- spans that errored
SELECT name, error
FROM mastra_ai_spans
WHERE error IS NOT NULL
ORDER BY startedAt DESC
LIMIT 10;
```

**Web dashboard.** You can also browse and query the database visually in the Turso dashboard at **https://app.turso.tech/** — sign in, then open org `tanzeelak` → database `find-food` (it has a data browser and a SQL console).

## Observability & metrics

Agent runs emit **traces and metrics** (duration, token usage, cost) automatically via Mastra's `Observability` + `MastraStorageExporter` (`src/mastra/index.ts`). Because libsql can persist traces but not metrics, the observability domain is routed to **DuckDB** (an OLAP store) through the composite store, while traces and everything else go to the default libsql store. Metrics now flow through end-to-end and show up in Studio.

### Local development

View metrics and traces in the local Studio dashboard:

```bash
npm run dev    # mastra dev — Studio at http://localhost:4111 -> Observability / Metrics
```

Two things to keep in mind:

- **`npm run cli` and `npm run dev` cannot run at the same time.** DuckDB allows only a single read-write process per file, so the observability DB can be held by exactly one process. The same applies to `npm run serve`. Start the one you need; stop it before starting another, or the second process fails with a DuckDB lock error.
- **Node 22+ is required** (the `mastra` CLI uses APIs unavailable in Node 18). Run `nvm use` first.

Metrics flush on a ~5s batch timer, so wait a few seconds after a run completes, and make sure the dashboard's time range includes "now".

To generate metrics from the CLI instead of the Studio playground:

1. Stop `npm run dev` (Ctrl-C) to release the DuckDB lock.
2. Run the CLI under Node 22 (`nvm use && npm run cli`) and chat a few turns. After the last response, **wait ~5s before typing `quit`** so the final turn's metrics flush.
3. Restart `npm run dev` to view the new metrics in Studio.

### Production

The project is deployed to the **Mastra platform** (see `.mastra-project.json` — project `food-agent`):

- **Deployed server / API:** https://food-agent.server.mastra.cloud/ — the running agent server (the prod equivalent of `npm run dev`); hosts the agent endpoints, including the AI SDK chat route `POST /chat/findFood` (point the frontend here via `NEXT_PUBLIC_MASTRA_URL`) and `POST /api/agents/findFood/generate`.
- **Deployed Studio:** https://food-agent.studio.mastra.cloud/ — the prod dashboard. Use **Observability → Traces / Metrics** to inspect production runs, plus the playground, logs, and memory.

You can also reach these from **https://projects.mastra.ai/** by selecting the `food-agent` project; `mastra studio deploy list` lists the current deployment URLs.

When `MASTRA_DB_URL` points at a shared cloud libsql (Turso) DB, local runs and the deployed app write traces to the same store, so prod and local share trace history.

## Scripts

```bash
npm run cli         # interactive CLI
npm run serve       # HTTP + SSE server
npm run dev         # Mastra dev playground
npm run db:shell    # open a Turso SQL shell against the find-food DB
npm run typecheck   # tsc --noEmit
npm run build       # tsc
```

## Notes

- Search uses Exa's hosted MCP server (`https://mcp.exa.ai/mcp`), not a direct REST client.
- Observability (traces + metrics) is built in via Mastra's storage exporter; traces persist to the default libsql store and metrics to DuckDB (see [Observability & metrics](#observability--metrics)). Mem0 is intentionally deferred; the current build relies on Mastra's built-in memory.
