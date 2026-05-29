# Go Backend

This is the current Codebuff-free runtime for Find Food.

It exposes a local HTTP API that accepts agent-like natural language requests, uses Exa for search, uses an OpenRouter/OpenAI-compatible LLM for structured extraction, and returns individual matching menu items.

## Run

From this directory:

```bash
GOCACHE="$PWD/.gocache" go run ./cmd/api
```

Health check:

```bash
curl http://127.0.0.1:3000/health
```

Find food:

```bash
curl -sS -X POST http://127.0.0.1:3000/api/find-food \
  -H 'Content-Type: application/json' \
  -d '{
    "message": "I want gluten-free fish tacos near me",
    "location": "Mission District SF",
    "dietaryRestrictions": ["gluten-free"]
  }'
```

## Test

```bash
GOCACHE="$PWD/.gocache" go test ./...
GOCACHE="$PWD/.gocache" go vet ./...
```

## Environment

The server reads `.env` from this directory or the repo root.

Required for real requests:

```bash
EXA_API_KEY=...
OPENROUTER_API_KEY=...
```

Optional:

```bash
LLM_PROVIDER=openrouter
LLM_MODEL=anthropic/claude-4-sonnet-20250522
PORT=3000
```
