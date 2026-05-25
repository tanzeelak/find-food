# System Architecture: find-food

## Overview

find-food is a two-agent system built on the [Codebuff](https://codebuff.com) platform that finds specific menu items — not just restaurants — matching a user's dietary restrictions. It uses a memory-first retrieval strategy to minimize redundant web searches and learns across sessions via Mem0.

---

## Agent Graph

```
User
 │
 ▼
find-food (orchestrator)
 ├── [MCP] Mem0 — resolve inputs + vetted restaurant lookup + write new finds
 ├── [MCP] Exa  — web search for new restaurant candidates
 └── spawn_agents ──► research-restaurant (N, parallel)
                           └── [MCP] Exa — menu lookup per restaurant
```

---

## Agents

### `find-food` — Orchestrator

| Property | Value |
|---|---|
| Model | `anthropic/claude-4-sonnet-20250522` |
| Tools | `spawn_agents` |
| MCP servers | Exa, Mem0 |
| Max steps | 25 (platform cap via `codebuff.json`) |

**Input schema**
- `params.location` (string, optional) — e.g. `"Mission District SF"`
- `params.dietaryRestrictions` (string[], optional) — e.g. `["gluten-free", "dairy-free"]`
- `prompt` (string, optional) — freeform context

**Decision logic (in order)**

1. **Resolve missing inputs from Mem0** — searches for location and dietary restrictions before prompting the user.
2. **Mem0 vetted-restaurant lookup** — searches previously-saved finds by neighborhood + cuisine.
   - 3+ matches → skip Exa entirely
   - 1–2 matches → supplement with Exa (top up to 5)
   - 0 matches → full Exa discovery
3. **Exa discovery** (when needed) — searches the web for candidate restaurants, then spawns `research-restaurant` subagents in parallel.
4. **Auto-save to Mem0** — persists a single consolidated memory of new Exa finds per run. Vetted spots are never re-saved. Transient context is never persisted.
5. **Return results** — up to 5 restaurants, each tagged `(from memory)` or `(new find)`.

---

### `research-restaurant` — Subagent

| Property | Value |
|---|---|
| Model | `x-ai/grok-4.3` |
| Tools | none |
| MCP servers | Exa |
| Spawned by | `find-food` |

**Input schema**
- `prompt` (string) — restaurant name
- `params.dietaryRestrictions` (string[]) — inherited from parent

**Output** — restaurant name, specific menu items meeting ALL restrictions, 10–20 word vibe, direct menu URL.

---

## External Services

| Service | Role | Auth |
|---|---|---|
| **Codebuff** | Agent runtime, orchestration, publishing | `CODEBUFF_API_KEY` |
| **Exa** (`mcp.exa.ai`) | Web retrieval — restaurant discovery + menu lookup | `EXA_API_KEY` (resolved at runtime per user) |
| **Mem0** (`mcp.mem0.ai`) | Persistent memory — preferences + vetted restaurant cache | `MEM0_API_KEY` (resolved at runtime per user) |
| **OpenRouter** | Model routing to Anthropic and xAI endpoints | implicit via Codebuff |

API keys are **not baked into the agent artifact** — Codebuff resolves `$ENV_VAR` placeholders from each end-user's local environment at runtime.

---

## Data Flow

```
1. User runs: codebuff find-food
2. find-food checks Mem0 for location + dietary restrictions
3. find-food queries Mem0 for vetted restaurants
4. [if needed] find-food queries Exa for new candidates
5. [if needed] find-food spawns research-restaurant per candidate (parallel)
6.             research-restaurant queries Exa for menu items
7.             research-restaurant returns structured result to find-food
8. find-food saves new finds to Mem0 (one consolidated write)
9. find-food returns formatted list to user (≤5 restaurants)
```

---

## Memory Architecture (Mem0)

### Read paths

| Query | Purpose |
|---|---|
| `"dietary restrictions"` / `"food allergies"` | Resolve missing `dietaryRestrictions` param |
| `"where do I live"` / `"home address"` | Resolve missing `location` param |
| `"<dish> in <neighborhood>"` | Find vetted restaurants matching request |

### Write paths

| Trigger | What gets written |
|---|---|
| New Exa finds | Single consolidated memory: name, neighborhood, cuisine, dietary fit, ordering tip |
| Explicit user opt-in (`"remember that..."`) | User preferences (dietary restrictions, home location, cuisine likes/dislikes) |

### Write constraints

- One `mem0_add` call per run (server-side extraction splits into per-restaurant facts and deduplicates)
- Transient context (current location, date, mood) is **never** persisted
- Preferences are **never** inferred — only written on explicit user opt-in

---

## Config Files

| File | Purpose |
|---|---|
| `.agents/find-food.ts` | find-food agent definition (model, tools, MCP, prompts, input schema) |
| `.agents/research-restaurant.ts` | research-restaurant agent definition |
| `.agents/types/agent-definition.ts` | Codebuff `AgentDefinition` TypeScript types |
| `.mcp.json` | MCP server config for local dev (Mem0 HTTP endpoint) |
| `codebuff.json` | Platform config: `maxAgentSteps: 25`, `addedSpawnableAgents: ["find-food"]` |
| `main.ts` | Codebuff SDK usage example (not part of the agent runtime) |
| `package.json` | Minimal dev dependency (`@types/node`) |

---

## Key Design Decisions

**Memory-first retrieval** — Mem0 is checked before any Exa call. With 3+ vetted matches the agent skips web search entirely, reducing latency and cost.

**Model selection per agent role** — The orchestrator uses Claude Sonnet (strong at reasoning and instruction-following); the researcher uses Grok (fast, good at web-grounded lookup). Routing through OpenRouter makes this swappable per agent.

**No publisher-side key baking** — All API keys use `$VAR` literal placeholders in agent definitions. Codebuff resolves them from the end-user's environment, so the published artifact never contains secrets.

**Single consolidated Mem0 write** — One `mem0_add` per run rather than one per restaurant. Mem0's server-side extraction splits and deduplicates the text, keeping write volume low.

**Parallel subagent spawning** — `research-restaurant` instances are spawned concurrently for all Exa candidates, keeping total latency bounded by the slowest single lookup rather than the sum.
