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
- Mem0 and tracing/observability (Arize) are intentionally deferred; the current build relies on Mastra's built-in memory.
