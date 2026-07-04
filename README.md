# Find your food

An AI-powered agent that finds specific menu items — not just restaurants — that match your dietary restrictions. Most apps stop at "vegetarian-friendly." This goes further, asking "can I actually eat this, and is it nourishing?" by scouring individual menus item by item.

> **Published:** find-food on Codebuff — 🥇 First place at the Codebuff Agent Builder Hackathon

![find-food screenshot](screenshot.webp)

---

## What it does

You describe what you're looking for — dietary restrictions, location, cuisine — and the agent:

1. Recalls your dietary profile from memory (restrictions, allergies, preferences)
2. Searches the web for restaurant candidates via Exa
3. Spawns a research subagent per restaurant to check actual menu items
4. Returns specific, orderable items with source-backed dietary fit, caveats, and ordering tips

Authenticated users get a persistent dietary profile that improves over time. Guests get an ephemeral session with no cross-session memory.

---

## Current Architecture

```
Browser
  └── Next.js frontend (Vercel)
        ├── Google OAuth → Supabase Auth
        ├── /api/access         — cross-user profile access management
        └── /api/chat/[agentId] — authenticated proxy to Mastra

Mastra backend (Mastra Server)
  └── POST /chat/findFood
        ├── findFood agent (orchestrator)
        │     ├── Exa MCP — web search for restaurants + menus
        │     └── researchRestaurant tool → bounded research subagent
        └── Mastra working memory (LibSQL/Turso, scope: "resource")

Supabase (PostgreSQL)
  ├── auth.users            — Google OAuth identity
  ├── public.profiles       — display name + email, auto-populated on sign-up
  └── public.profile_access — cross-user read access grants (RLS enforced)
```

See [architecture.md](architecture.md) for a full breakdown of the system design, data flow, and key decisions.

## Project Layout

```
app/
  src/proxy.ts                              Next.js middleware — refreshes Supabase session cookies
  src/app/auth/callback/route.ts            Google OAuth code → Supabase session exchange
  src/app/api/chat/[agentId]/route.ts       Authenticated proxy: injects resourceId + JWT, forwards to Mastra
  src/app/api/access/route.ts               Cross-user access management (grant / revoke / list)
  src/app/page.tsx                          Home — reads Supabase user server-side
  src/components/assistant.tsx             Chat UI — side-by-side panes, profile toggle pills
  src/components/access-settings.tsx       Manage access modal (grant / revoke grantees)
  src/components/login-form.tsx            Google sign-in button
  src/lib/supabase/server.ts               Server-side Supabase client (cookies)
  src/lib/supabase/client.ts               Browser Supabase client

orchestra/
  src/mastra/index.ts                      Mastra instance, agents, storage, auth, custom routes
  src/mastra/agents/find-food.ts           Orchestrator agent
  src/mastra/agents/research-restaurant.ts Research subagent
  src/mastra/tools/research-restaurant.ts  Tool that spawns the research agent
  src/mastra/memory/index.ts               Working memory config (LibSQL, scope: "resource")
  src/mastra/prompts.ts                    Agent system prompts
  src/mastra/model.ts                      Model configuration
  src/cli.ts                               Local interactive CLI (runs agent in-process)

legacy/
  .agents/                                 Original Codebuff agent definitions (reference only)
  architecture.md                          Legacy Codebuff architecture notes
```

---

## Dev

### Frontend

```bash
cd app
nvm use && npm install
npm run dev        # http://localhost:3000
```

`app/.env.local`:

```bash
MASTRA_URL=http://localhost:4111
NEXT_PUBLIC_SUPABASE_URL=...
NEXT_PUBLIC_SUPABASE_ANON_KEY=...
```

### Backend

```bash
cd orchestra
nvm use && npm install
npm run dev        # http://localhost:4111
```

`orchestra/.env`:

```bash
EXA_API_KEY=...
OPENROUTER_API_KEY=...
SUPABASE_URL=...
SUPABASE_ANON_KEY=...
MASTRA_DB_URL=libsql://<db>.<region>.turso.io   # optional: shared Turso instead of local SQLite
MASTRA_DB_AUTH_TOKEN=...
```

CLI (runs the agent in-process without an HTTP server):

```bash
cd orchestra && npm run cli
```

---

## Prod

### Frontend

Hosted on **Vercel** — `https://find-food-kohl.vercel.app/`

Deploy by pushing to `main`. Set these environment variables in the Vercel project:

```bash
MASTRA_URL=https://food-agent.server.mastra.cloud
NEXT_PUBLIC_SUPABASE_URL=...
NEXT_PUBLIC_SUPABASE_ANON_KEY=...
```

Add your production URL to the Supabase redirect allowlist: `https://your-domain.com/auth/callback`

Do not add backend secrets (`EXA_API_KEY`, `OPENROUTER_API_KEY`, `MASTRA_DB_*`) to the Vercel project.

### Backend

Hosted on **Mastra Server** — `https://food-agent.server.mastra.cloud/`

```bash
cd orchestra
mastra server deploy

# If env vars changed:
mastra server env import .env
```

Smoke test:

```bash
curl https://food-agent.server.mastra.cloud/api/agents
curl -i -X POST https://food-agent.server.mastra.cloud/chat/findFood
```

---

## Database

### Mastra (Turso / LibSQL)

Stores agent working memory, thread history, and observability spans.

- **Production**: `libsql://find-food-tanzeelak.aws-us-east-2.turso.io`
- **Local dev**: SQLite files under `orchestra/.mastra/` (auto-created)
- Credentials: `MASTRA_DB_URL` + `MASTRA_DB_AUTH_TOKEN` in `orchestra/.env`

Schema is managed by Mastra automatically (no migrations to run manually).

### Supabase (PostgreSQL)

Stores user identity and cross-user access grants.

- **Project**: `tahcqfcxohlqrnzclovr.supabase.co`
- Tables: `auth.users` (managed by Supabase), `public.profiles`, `public.profile_access`
- RLS is enabled on all public tables — policies enforce that users can only read/write their own data
- Schema changes must be run manually in the **Supabase SQL Editor**

---

## Observability

### Mastra

Traces and agent runs are visible in **Mastra Studio**: `https://food-agent.studio.mastra.cloud`

Deploy Studio separately when the dashboard bundle needs updating:

```bash
mastra studio deploy --project food-agent
```

Observability data (spans, traces) is stored in a local DuckDB file (`orchestra/.mastra/find-food-observability.duckdb`) in dev, and on the Mastra Server in prod.

### Supabase

User signups, active sessions, and auth events are visible in the **Supabase dashboard** under Authentication → Users and Auth → Logs. Row-level access to `profiles` and `profile_access` can be inspected in the Table Editor.

### Google OAuth

OAuth consent and sign-in activity is visible in the **Google Cloud Console** under APIs & Services → Credentials → your OAuth 2.0 client. Quota usage and error rates appear under APIs & Services → Google Identity.
