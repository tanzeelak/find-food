# Migration Plan: Move Off Codebuff

## Goal

Move `find-food` from a Codebuff-hosted agent definition into an owned backend + frontend architecture while keeping the user experience agent-like:

> "I want fish tacos near me that work with my restrictions."

The product should still accept natural language, resolve missing context, search for restaurants, inspect menus, and return specific food items. The implementation should make control flow, memory writes, frontend integration, and observability explicit.

## Target Architecture

```txt
Frontend
  -> Backend API
      -> CoreFoodAgent
          -> ask follow-up or call find_menu_items
          -> backend find_menu_items tool
          -> Exa restaurant discovery
          -> menu/source extraction LLM calls
          -> optional bounded agents for complex research
          -> Mem0 profile + restaurant memory
          -> OpenTelemetry / Arize tracing
```

Core principle:

```txt
Code owns policy and workflow.
LLMs interpret and extract.
Agents handle bounded, messy research tasks when plain LLM calls are not enough.
```

## Phase 1: MVP - Agent-Like Prompting With Deterministic Backend

Build the first non-Codebuff version as a normal backend workflow. The user input should still feel conversational, but the backend should own the steps.

### User Experience

The user can submit natural language like:

```txt
I want gluten-free fish tacos near me
```

The system should return specific menu items that match the dietary restrictions, with restaurant details attached to each item.

### Backend Workflow

1. Accept a request:
   - `message`
   - optional `location`
   - optional `dietaryRestrictions`
2. Use an LLM structured-output call to parse intent:
   - food query / craving
   - explicit location, if any
   - whether "near me" was requested
   - dietary restrictions mentioned in the prompt
   - missing required fields
3. If required fields are missing, return a follow-up question instead of guessing.
4. Search Exa for candidate restaurants.
5. Fetch likely menu pages or relevant source pages.
6. Use LLM calls to isolate:
   - menu items
   - dietary fit
   - dietary accommodations
   - source URLs
7. Flatten matching menu items across researched restaurants.
8. Rank and dedupe item results.
9. Return structured JSON to the frontend.

### MVP Components

```txt
orchestra/cmd/api/
  main.go
orchestra/internal/
  agent/
    workflow.go
    prompts.go
    types.go
  api/
    server.go
  exa/
    client.go
  llm/
    client.go
```

Phase 1 is implemented in Go under `orchestra/`. The frontend can remain separate and call the Go API over HTTP.

### MVP Response Shape

```json
{
  "status": "complete",
  "items": [
    {
      "name": "Item 1",
      "restaurantName": "Restaurant Name",
      "restaurantSource": "new_find",
      "distanceText": "~8 min walk",
      "whyItFits": "source-backed dietary evidence",
      "caveats": ["cross-contamination caveat if any"],
      "dietaryAccommodations": ["Accommodation 1"],
      "menuUrl": "https://example.com/menu",
      "sourceUrls": ["https://example.com/menu"],
      "confidence": "medium"
    }
  ],
  "followUpQuestion": null
}
```

If the prompt is underspecified:

```json
{
  "status": "needs_input",
  "items": [],
  "followUpQuestion": "What location should I search near?"
}
```

### MVP Non-Goals

- No Mem0 yet.
- No open-ended autonomous tool loop beyond the bounded core food agent.
- No memory writes.
- No full observability beyond normal logs.
- No Codebuff dependency.

### Exit Criteria

- A frontend or API client can submit one natural-language request.
- Backend returns structured restaurant/menu-item JSON.
- Exa is called by backend service code, not by Codebuff.
- Agent decisions and LLM extraction calls are narrow and schema-validated.
- Tool execution path is deterministic enough to debug from logs.

## Phase 2: Replace Complex LLM Calls With Bounded Agents

Once the MVP works, identify the steps where single-shot LLM calls are brittle. Replace only those steps with bounded agents.

### Candidate Agent: ResearchRestaurantAgent

Purpose:

```txt
Given one restaurant, a food query, and dietary restrictions, find reliable menu evidence and return structured results.
```

Allowed tools:

```txt
exa.search
exa.fetch
```

Not allowed:

```txt
mem0.add
mem0.delete
arbitrary network access
profile mutation
```

Runtime limits:

- Max 3-5 tool calls per restaurant.
- Timeout per restaurant.
- Structured JSON output only.
- Source URLs required.
- Failed research returns a typed failure, not freeform text.

### Agent Output

```json
{
  "restaurantName": "La Taqueria",
  "hasSuitableItems": true,
  "menuItems": [
    {
      "name": "Fish taco on corn tortilla",
      "whyItFits": "Corn tortilla, no dairy ingredients found",
      "caveats": ["Ask about shared fryer"]
    }
  ],
  "dietaryAccommodations": ["Corn tortillas available"],
  "menuUrl": "https://example.com/menu",
  "sourceUrls": ["https://example.com/menu"],
  "confidence": "medium"
}
```

### Workflow After Phase 2

```txt
FindFoodWorkflow
  -> parse intent with LLM call
  -> Exa discovery service
  -> ResearchRestaurantAgent[] in parallel
  -> deterministic ranking / formatting
```

### Exit Criteria

- Complex menu research is handled by a bounded agent.
- The top-level workflow still owns policy and ordering.
- Agent runs are traceable and cancellable.
- Agent failures do not fail the entire request unless all candidates fail.

## Phase 3: Add Mem0

Add memory after the bounded agent/tool workflow and research behavior are working. Memory should improve relevance without making the system unpredictable.

### Read Paths

Use Mem0 to resolve missing context:

- dietary restrictions
- food allergies
- home location or preferred search area
- previously vetted restaurants for a neighborhood / dish / restriction set

### Write Paths

Only write durable restaurant findings and explicit user preferences.

Allowed auto-write:

```txt
Vetted gluten-free + dairy-free fish taco spots near Mission SF:
Restaurant A (safe item + ordering tip), Restaurant B (...)
```

Disallowed auto-write:

- transient location
- mood
- one-time craving
- inferred medical restriction
- anything from an untrusted source without backend validation

### Updated Workflow

```txt
FindFoodWorkflow
  -> parse intent
  -> resolve missing fields from Mem0
  -> search Mem0 for vetted restaurants
  -> if 3+ strong vetted matches, skip Exa
  -> otherwise search Exa and research candidates
  -> save only new durable finds to Mem0
  -> return structured JSON
```

### Memory Policy

Code owns memory writes. Agents and LLM calls may recommend a memory write, but only backend policy code decides whether `mem0.add` is allowed.

### Exit Criteria

- User can omit known dietary restrictions and location.
- Vetted Mem0 results can short-circuit Exa.
- New finds are saved once per run in a consolidated write.
- No transient context is written automatically.

## Phase 4: Add Observability

Add OpenTelemetry tracing and export traces to Arize. Observability should cover the workflow, LLM calls, agent runs, MCP/service calls, and frontend request lifecycle.

### Trace Shape

```txt
find_food.request
  -> intent.parse
  -> mem0.resolve_inputs
  -> mem0.search_vetted
  -> exa.discovery_search
  -> restaurant.research
      -> agent.step
      -> exa.search
      -> exa.fetch
      -> llm.extract_menu_items
  -> mem0.save_new_finds
  -> response.rank_and_format
```

### Span Attributes

Safe attributes:

- request id
- trace id
- candidate count
- vetted result count
- new find count
- model name
- tool name
- latency
- status
- error type

Avoid:

- API keys
- full home address
- raw user medical details
- full fetched page text
- unredacted prompts if they contain sensitive personal data

### Frontend Integration

Return `traceId` in the API response:

```json
{
  "traceId": "abc123",
  "status": "complete",
  "items": []
}
```

The frontend can display the trace id in a debug area or attach it to bug reports.

### Exit Criteria

- Every user request has one top-level trace.
- LLM calls and agent tool calls are visible as child spans.
- Exa and Mem0 latency/errors are visible.
- API responses include a trace id.
- Sensitive fields are redacted before export.

## Suggested Implementation Order

1. Decide backend language: Go or TypeScript.
2. Create backend API skeleton.
3. Port prompts from `.agents/find-food.ts` and `.agents/research-restaurant.ts`.
4. Define request/response schemas.
5. Implement Exa service.
6. Implement core food agent decision call.
7. Implement menu extraction LLM call.
8. Add simple frontend form and result view.
9. Introduce `ResearchRestaurantAgent`.
10. Add Mem0 reads and writes.
11. Add OpenTelemetry + Arize.
12. Remove Codebuff runtime dependency when parity is good enough.

## Open Questions

- Backend language: Go or TypeScript?
- Model provider: OpenRouter, OpenAI directly, Anthropic directly, or mixed?
- Should "near me" use browser geolocation, Mem0 home location, or both?
- Should Mem0 be accessed through MCP, REST, or SDK?
- What is the minimum acceptable citation/source requirement for menu claims?
- How strict should dietary compatibility be when source text is ambiguous?
