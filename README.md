# Find _your_ food

Find Food helps identify specific menu items that match dietary restrictions. The project is migrating off the original Codebuff agent runtime into an owned Go backend.

![find-food screenshot](screenshot.webp)

## Status

Current runtime:

- `orchestra/` - active Go backend
- HTTP API: `POST /api/find-food`
- Uses a core food agent for conversation/tool choice, Exa for search, and OpenRouter-compatible LLM calls for structured extraction
- Returns individual matching menu items, not restaurant-only recommendations

Legacy runtime:

- `codebuff/` - original Codebuff implementation
- Kept for prompt, architecture, and behavior reference during migration

## Project Layout

```txt
orchestra/
  cmd/api/              current API entrypoint
  internal/agent/       core agent, backend tools, prompts, and response types
  internal/api/         HTTP routes
  internal/exa/         Exa search client
  internal/llm/         OpenRouter/OpenAI-compatible LLM client

codebuff/
  .agents/              legacy Codebuff agent definitions
  codebuff.json         legacy Codebuff runtime config
  main.ts               Codebuff SDK example

plan.md                migration plan
codebuff/architecture.md original Codebuff architecture notes
```

## Run The Go Backend

From the repo root:

```bash
cd orchestra
GOCACHE="$PWD/.gocache" go run ./cmd/api
```

Health check:

```bash
curl http://127.0.0.1:3000/health
```

Find food request:

```bash
curl -sS -X POST http://127.0.0.1:3000/api/find-food \
  -H 'Content-Type: application/json' \
  -d '{
    "message": "I want gluten-free fish tacos near me",
    "location": "Mission District SF",
    "dietaryRestrictions": ["gluten-free"]
  }'
```

The server logs progress while a request is running: core agent decision, `find_menu_items` tool execution, Exa discovery, candidate extraction, and per-restaurant menu research.

## Response Shape

Successful responses are item-centric:

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

## Environment

The Go backend reads `.env` from `orchestra/.env` or the repo-root `.env`.

Required for real requests:

```bash
EXA_API_KEY=your_exa_api_key
OPENROUTER_API_KEY=your_openrouter_api_key
```

Optional:

```bash
LLM_PROVIDER=openrouter
LLM_MODEL=anthropic/claude-4-sonnet-20250522
PORT=3000
```

## Test

```bash
cd orchestra
GOCACHE="$PWD/.gocache" go test ./...
GOCACHE="$PWD/.gocache" go vet ./...
```

## Legacy Codebuff Flow

The Codebuff implementation remains available in `codebuff/`.

```bash
cd codebuff
npm install
codebuff find-food
```

See `codebuff/README.md` for the legacy folder contents.
