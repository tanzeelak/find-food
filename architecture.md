# Architecture: Find Food

Find Food is a conversational app that finds specific, orderable menu items matching a user's dietary restrictions near a location. It is built as two separate deployments — a Next.js frontend and a Mastra AI backend — connected via an authenticated HTTP proxy.

---

## System Overview

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
        │     └── researchRestaurant tool → research agent (bounded)
        └── Mastra working memory (LibSQL/Turso, scope: "resource")

Supabase (PostgreSQL)
  ├── auth.users            — Google OAuth identity
  ├── public.profiles       — display name + email, auto-populated on sign-up
  └── public.profile_access — cross-user read access grants (RLS enforced)
```

---

## Authentication

### Google OAuth via Supabase

1. User clicks "Sign in with Google" → calls `supabase.auth.signInWithOAuth({ provider: "google" })`
2. Google redirects to `/auth/callback` with an OAuth code
3. `/auth/callback` exchanges the code for a Supabase session via `supabase.auth.exchangeCodeForSession(code)`
4. Supabase issues a JWT (access token) and sets it as a cookie
5. `proxy.ts` (Next.js middleware) refreshes the session cookie on every request using `@supabase/ssr`

### JWT forwarding to Mastra

The Next.js chat proxy (`/api/chat/[agentId]/route.ts`) reads the Supabase session server-side and forwards the JWT to Mastra:

```
Authorization: Bearer <supabase-access-token>
```

Mastra's `@mastra/auth-supabase` validates the JWT on protected routes. The `/chat/` routes are intentionally public so guests can also use the app without signing in.

### Guest mode

Unauthenticated users get a fresh random UUID (generated client-side with `crypto.randomUUID()`) as their `resourceId`. This is regenerated on every page load — guest memory is intentionally ephemeral and never persists across sessions.

---

## Resource Identity

Every conversation is keyed to a `resourceId`:

| User type | resourceId | Persistence |
|---|---|---|
| Authenticated | `session.user.id` (Supabase UUID) | Permanent — survives across sessions |
| Guest | `crypto.randomUUID()` per page load | Ephemeral — lost on refresh |

The Next.js proxy injects `resourceId` server-side, overriding whatever the client sent. Authenticated users cannot spoof another user's `resourceId`.

---

## Agent Architecture

### `findFood` — Orchestrator

The primary agent. Receives the user's message and manages the full search loop.

- **Model**: configured via OpenRouter (see `orchestra/src/mastra/model.ts`)
- **Tools**: `researchRestaurant` — spawns a bounded research agent for a specific restaurant
- **MCP**: Exa — web search for restaurant discovery
- **Memory**: Mastra working memory (scope: `"resource"`) — stores the user's dietary profile, home location, food preferences across sessions

**Decision logic**:
1. Resolve dietary restrictions and location from working memory if not stated
2. Search Exa for restaurant candidates matching the request
3. For each candidate, call `researchRestaurant` to get specific menu items
4. Return results with source-backed dietary fit, caveats, and ordering tips

### `researchRestaurant` — Subagent

A bounded agent spawned by the orchestrator per restaurant candidate.

- **Model**: configured separately (can differ from orchestrator)
- **Tools**: none
- **MCP**: Exa — menu and dietary information lookup per restaurant
- **Output**: restaurant name, specific menu items, vibe summary, menu URL

---

## Memory Architecture

Working memory uses Mastra's built-in memory system with `scope: "resource"`. This means a single memory record is maintained per user (keyed by their Supabase UUID) and persists across all their chat threads.

**Storage**: LibSQL / Turso (`find-food-tanzeelak.aws-us-east-2.turso.io`)

**Working memory template** (initial state for new users):
```
# User Food Profile
- Dietary restrictions:
- Food allergies:
- Home / usual search location:
- Food likes:
- Food dislikes:
```

The agent updates this template as it learns about the user through conversation. The `update-working-memory` tool is called automatically by Mastra when the agent decides to persist new facts.

---

## Cross-User Read-Only Access

Authenticated users can grant other users read access to their dietary profile. This allows user B to use the agent to find food *for* user A, using A's restrictions — without being able to modify A's profile.

### Supabase schema

```sql
-- Auto-populated when a user signs up (trigger on auth.users insert)
public.profiles (id, email, display_name, created_at)

-- Access grants: owner grants grantee read access to their profile
public.profile_access (owner_id, grantee_id, created_at)
-- RLS: owner/grantee can SELECT; only owner can INSERT/DELETE
```

### Access management API

| Method | Route | Description |
|---|---|---|
| `GET` | `/api/access` | Profiles that have granted me access (I'm the grantee) |
| `GET` | `/api/access?as=owner` | People I've granted access to my profile (I'm the owner) |
| `POST` | `/api/access` | Grant my profile access to someone by email |
| `DELETE` | `/api/access` | Revoke a grant by granteeId |

### Read-only enforcement

When the frontend sends `targetResourceId` (the owner's ID), the chat proxy:

1. Verifies a `profile_access` row exists (`owner_id=targetResourceId`, `grantee_id=me`) in Supabase
2. Returns 403 if not
3. Fetches the owner's working memory from the Mastra server (`GET /working-memory/:resourceId`)
4. Injects it as a read-only context message at the start of the conversation
5. Sets `memory.resource = grantee_id` (not the owner's ID)

Step 5 is the key to read-only enforcement: writes from the conversation go to the grantee's own memory, never to the owner's.

### Frontend

The header shows a toggle pill (`+ Name` / `− Name`) for each profile the current user has been granted access to. Toggling opens a side-by-side chat pane using that profile's dietary context. The user's own pane is always visible and cannot be closed.

The "Manage access" modal lets owners:
- See who currently has access to their profile
- Grant access to a new user by email
- Revoke access from an existing grantee

---

## Data Flow: Authenticated Request

```
1. User sends message in browser
2. ChatPanel sends POST /api/chat/findFood
     body: { messages, memory: { thread, resource: user.id } }
     (resource may be overridden to targetResourceId for cross-user panes)

3. Next.js proxy (/api/chat/[agentId]/route.ts):
     a. Reads Supabase session server-side
     b. If targetResourceId: verifies grant in Supabase, fetches owner's
        working memory from Mastra, injects as context message
     c. Sets memory.resource = session.user.id (grantee) or own ID
     d. Forwards request to Mastra with Authorization: Bearer <jwt>

4. Mastra server (POST /chat/findFood):
     a. Validates JWT (optional — /chat/ is public, guests pass through)
     b. Loads working memory for memory.resource from LibSQL
     c. Runs findFood agent with memory context injected into system prompt
     d. Agent calls Exa MCP and/or researchRestaurant tool
     e. Agent updates working memory if new facts learned
     f. Streams response back via SSE

5. Next.js proxy streams response to browser
6. assistant-ui renders message tokens as they arrive
```

---

## External Services

| Service | Role | Credentials |
|---|---|---|
| **Supabase** | Auth (Google OAuth), user profiles, access control (RLS) | `NEXT_PUBLIC_SUPABASE_URL`, `NEXT_PUBLIC_SUPABASE_ANON_KEY` |
| **Google OAuth** | Identity provider — sign-in with Google | Configured in Supabase dashboard |
| **Exa** | Web search — restaurant discovery + menu lookup | `EXA_API_KEY` (Mastra backend only) |
| **OpenRouter** | Model routing to Anthropic and other providers | `OPENROUTER_API_KEY` (Mastra backend only) |
| **Turso / LibSQL** | Agent memory + thread storage + observability | `MASTRA_DB_URL`, `MASTRA_DB_AUTH_TOKEN` |
| **Vercel** | Frontend hosting | GitHub integration |
| **Mastra Server** | Backend hosting for the AI agents | `MASTRA_PLATFORM_ACCESS_TOKEN` |

Frontend secrets: only `NEXT_PUBLIC_SUPABASE_URL` and `NEXT_PUBLIC_SUPABASE_ANON_KEY` (both safe to expose).
Backend secrets: `EXA_API_KEY`, `OPENROUTER_API_KEY`, `MASTRA_DB_*` — never in the frontend.