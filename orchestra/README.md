# Go Backend

This is the current Codebuff-free runtime for Find Food.

It exposes a local HTTP API backed by a core food agent. The agent manages conversation state, decides whether to ask a follow-up or call the `find_menu_items` backend tool, uses Exa for search, uses an OpenRouter/OpenAI-compatible LLM for structured extraction, and returns individual matching menu items.

## Run

From this directory:

```bash
GOCACHE="$PWD/.gocache" go run ./cmd/api
```

Health check:

```bash
curl http://127.0.0.1:3000/health
```

Chat client:

With the API server running, start a terminal chat session in another terminal:

```bash
go run ./cmd/chat
```

The chat client keeps `conversationId` for you. Type `reset` to start over and
`quit` to exit.

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

Agent conversation:

```bash
curl -sS -X POST http://127.0.0.1:3000/api/find-food \
  -H 'Content-Type: text/plain' \
  --data 'I want gluten-free fish tacos near me'
```

If the response asks a follow-up, send the returned `conversationId` back with
the next turn:

```bash
curl -sS -X POST http://127.0.0.1:3000/api/find-food \
  -H 'Content-Type: text/plain' \
  -H 'X-Conversation-ID: <conversationId>' \
  --data 'Mission District SF'
```

You can also include `conversationId` in the JSON body instead of using the
header.

Responses are pretty-printed JSON by default so direct `curl` output is readable.

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
