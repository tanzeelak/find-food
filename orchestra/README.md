# orchestra

Mastra AI backend for Find Food. See the [root README](../README.md) for setup, deployment, and environment variables, and [architecture.md](../architecture.md) for the full system design.

## Tech stack

| Concern | Package |
|---|---|
| Agent framework | `@mastra/core` |
| Model provider | `@openrouter/ai-sdk-provider` |
| Web search | Exa via `@mastra/mcp` (hosted MCP server) |
| Chat API | `@mastra/ai-sdk` — `chatRoute()` at `POST /chat/:agentId` |
| Memory | `@mastra/memory` + `@mastra/libsql` |
| Storage | `@mastra/libsql` (default) + `@mastra/duckdb` (observability domain) via `MastraCompositeStore` |
| Observability | `@mastra/observability` + `MastraStorageExporter` |
| Auth | `@mastra/auth-supabase` |
| Validation | `zod` |
| Runtime | Node ≥ 20 (see `.nvmrc`) |

## Project layout

```
src/
  cli.ts                       Interactive CLI (readline → agent stream → stdout)
  mastra/
    index.ts                   Mastra instance — agents, storage, auth, custom routes
    run.ts                     runFindFoodTurn() — transport-agnostic entry point
    agents/
      find-food.ts             Orchestrator agent
      research-restaurant.ts   Bounded research subagent
    tools/
      research-restaurant.ts   Tool wrapping the research agent
    mcp/
      exa.ts                   Exa MCP client + tool selection
    memory/
      index.ts                 Working memory config (LibSQL, scope: "resource")
    model.ts                   OpenRouter model wiring
    prompts.ts                 Agent system prompts
    schemas.ts                 Zod schemas (research input/result, menu items)
    env.ts                     .env loading + path helpers
```

## CLI

```bash
npm run cli
```

```
you > gluten-free fish tacos in the Mission District, San Francisco
```

Commands: `reset` (new thread), `quit` / `exit`.

## Turso SQL queries

Useful for inspecting memory and traces. Run via `npm run db:shell` (opens a shell against the `find-food` Turso DB):

```sql
-- saved user profiles (working memory), one row per resourceId
SELECT id, workingMemory FROM mastra_resources;

-- recent conversations
SELECT id, resourceId, title, createdAt FROM mastra_threads ORDER BY createdAt DESC LIMIT 10;

-- full transcript of one thread
SELECT role, createdAt, content FROM mastra_messages WHERE thread_id = '<id>' ORDER BY createdAt;

-- recent agent run traces
SELECT name, startedAt, endedAt FROM mastra_ai_spans ORDER BY startedAt DESC LIMIT 20;

-- errored spans
SELECT name, error FROM mastra_ai_spans WHERE error IS NOT NULL ORDER BY startedAt DESC LIMIT 10;
```

## Notes

- **CLI and `npm run dev` cannot run simultaneously.** DuckDB allows only one read-write process per file. Stop one before starting the other or the second will fail with a lock error.
- Metrics flush on a ~5s timer — wait a few seconds after a run before quitting or checking Studio.
- Traces go to libsql (Turso); metrics go to DuckDB. Both are wired via `MastraCompositeStore`.
