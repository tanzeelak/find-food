# Find _your_ food
An AI-powered agent that finds specific menu items, not just restaurants, that match your dietary restrictions. Most apps stop at "vegetarian-friendly." This goes further, asking "can I actually eat this, and is it nourishing?" by scouring individual menus item by item.

**Published:** [find-food on Codebuff](https://www.codebuff.com/publishers/tanzeela/agents/find-food/0.0.5) — 🥇 First place at the Codebuff Agent Builder Hackathon
![find-food screenshot](screenshot.webp)

## How it works

Two agents collaborate:

1. **`find-food`** — the orchestrator. Given a location and a list of dietary restrictions, it first checks the [Mem0](https://mem0.ai) MCP for previously-vetted restaurants matching the request (cuisine/dish + neighborhood). With 3+ vetted matches in walking distance, it skips fresh discovery entirely. With 1–2 matches, it supplements via [Exa](https://exa.ai) to top up to 5 candidates. With 0, it runs a full Exa search and spawns a `research-restaurant` subagent per Exa candidate. Each restaurant in the output is tagged `(from memory)` or `(new find)` so you can see the source. If `location` or `dietaryRestrictions` are omitted, they're also resolved from Mem0 first, with the user prompted only as a last resort. New finds discovered via Exa are auto-saved back to Mem0 so the next similar request hits the cache.

2. **`research-restaurant`** — the researcher. For a single restaurant, it uses Exa to check whether the menu has items that satisfy *all* of the specified dietary restrictions. Returns the restaurant name, matching menu items, a short vibe description, and a direct link to the menu.

## Requirements

- [Codebuff](https://codebuff.com) account + API key
- [Exa](https://exa.ai) API key (primary retrieval) or [Yelp API](https://docs.developer.yelp.com/) key (fallback)
- [OpenRouter](https://openrouter.ai) API key (for model selection)
- [Mem0](https://mem0.ai) API key (used by `find-food` to recall stored location and dietary preferences across sessions)

## Setup

```bash
npm install
```

Create a `.env` file:

```
CODEBUFF_API_KEY=your_codebuff_api_key
EXA_API_KEY=your_exa_api_key
MEM0_API_KEY=your_mem0_api_key
```

## Phase 1 Go backend

The Codebuff-free MVP backend lives under `cmd/api` and `internal/*`.

Run tests:

```bash
go test ./...
```

Start the API:

```bash
go run ./cmd/api
```

The server starts on `http://127.0.0.1:3000` by default.

Health check:

```bash
curl http://127.0.0.1:3000/health
```

Find food request:

```bash
curl -X POST http://127.0.0.1:3000/api/find-food \
  -H 'Content-Type: application/json' \
  -d '{
    "message": "I want gluten-free fish tacos near me",
    "location": "Mission District SF",
    "dietaryRestrictions": ["gluten-free"]
  }'
```

Successful responses are item-centric. The backend may search candidate restaurants internally, but it returns individual menu items that match the dietary restrictions:

```json
{
  "status": "complete",
  "items": [
    {
      "name": "Fish Tacos (Gluten Free)",
      "restaurantName": "Lolo",
      "restaurantSource": "new_find",
      "whyItFits": "Explicitly labeled gluten-free fish tacos",
      "caveats": ["Cross-contamination possible"],
      "dietaryAccommodations": ["Gluten-free items marked on menu"],
      "menuUrl": "https://example.com/menu",
      "sourceUrls": ["https://example.com/menu"],
      "confidence": "high"
    }
  ],
  "followUpQuestion": null
}
```

Required environment variables for the real workflow:

```bash
EXA_API_KEY=your_exa_api_key
OPENROUTER_API_KEY=your_openrouter_api_key
```

Optional model settings:

```bash
LLM_PROVIDER=openrouter
LLM_MODEL=anthropic/claude-4-sonnet-20250522
PORT=3000
```

### Getting a Mem0 API key

The easiest path is the Mem0 CLI's agent-mode signup:

```bash
npm install -g @mem0/cli
mem0 init --agent --agent-caller codebuff
```

This signs you up and configures the CLI locally. Confirm with `mem0 whoami`. To wire the key into this project, copy it from the [Mem0 dashboard](https://mem0.ai) (or from wherever your installed CLI version persists it under `~/.mem0/`) into `.env` as `MEM0_API_KEY`. To later transfer ownership of the auto-provisioned account to a real email, run `mem0 init --email <your-email>` — this upgrades the identity in place; verify against the latest [Mem0 docs](https://docs.mem0.ai) for the current behavior around key and memory migration.

Alternatively, sign up at [mem0.ai](https://mem0.ai) and create a key from the dashboard directly.

## Usage

```bash
codebuff find-food
```

When prompted, provide:
- **Location** — e.g. `"Mission District SF"` or `"Downtown Portland"`
- **Dietary restrictions** — e.g. `["gluten-free", "dairy-free", "pescatarian"]`

Each result should identify a specific menu item that matches your dietary restrictions, plus the restaurant, evidence, caveats, and menu/source links.

### Skip the prompts with Mem0

If you've stored your home location and dietary restrictions in Mem0 (via `mem0 add "I live at ..."` and `mem0 add "My dietary restrictions are: ..."`), `find-food` will pull them automatically when you omit those params.

**Write contract:** `find-food` writes to Mem0 in two narrow cases:

1. **Auto-save of new restaurant finds.** When Exa discovers restaurants you haven't seen before, the agent persists a single consolidated memory of those finds (name, neighborhood, dietary fit, ordering tip) so the next similar request can serve them from cache. Vetted spots already in memory are not re-saved.
2. **Explicit user opt-in for preferences.** Stable preferences (dietary restrictions, home location, cuisine likes/dislikes) are only persisted when you explicitly say so — e.g. *"remember that I just went vegan"* or *"save this: I'm allergic to shellfish"*.

The agent **never** persists transient context ("I'm at X right now", your current mood, dates) and **never** writes inferred preferences without an opt-in cue.

## What I learned

**Models need to translate fuzzy human intent.** Dietary restrictions are a good stress test for this: they're nuanced and overlapping. Getting a model to correctly interpret "I can't eat this" vs. "I prefer not to eat this" is non-trivial, and it matters a lot for utility.

**Retrieval quality matters more than tooling familiarity.** Exa outperformed Yelp significantly for this use case, even though Yelp is the obvious choice. Exa was harder to reason about as a human, but the results were better, which is a good reminder that the right tool isn't always the familiar one.

**Reliability is a moving target with evolving tools.** Exa ran into network issues, which forced a temporary swap to the [Yelp API](https://docs.developer.yelp.com/). Agentic systems need fallback strategies baked in.

**Model cost should scale with restriction complexity.** I used [OpenRouter](https://openrouter.ai) to route between models depending on the complexity of a user's restrictions. Someone with highly specific restrictions benefits from a more capable (and more expensive) model. Someone with simple preferences doesn't need that. A one-size-fits-all model choice leaves value on the table in both directions.

**Use only as much structure as you need.** [Codebuff](https://codebuff.com) is a structured agent builder. It makes it easy to define task types, spawn subagents, and share tools. Only some of that structure was genuinely useful here. Finding the minimum viable structure is its own design problem.

**Consistency is an open question.** Results varied noticeably across runs, even though restaurant menus are largely static. The source of that variability is worth digging into.

## Planned improvements
- Auto switch models. Switch to Grok 4.3
- Rotate API keys regularly
- Let users set dietary restrictions interactively
- Investigate result consistency across runs
- Make location finder more context specific
- Add a per-restaurant cache to `research-restaurant` (keyed by restaurant + restrictions, with a TTL) so menu lookups are reused across `find-food` runs
- Add a TTL/freshness check to the Mem0-vetted lookup so closed restaurants and stale notes are surfaced for re-validation
- Deploy this onto a frontend
    - Frontend reads/writes the same Mem0 profile the CLI uses, so preferences and vetted restaurants are shared across surfaces
