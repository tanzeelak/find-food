import type { AgentDefinition } from './types/agent-definition'

const definition: AgentDefinition = {
  id: 'find-food',
  displayName: 'Find Food',
  publisher: "tanzeela",

  model: 'anthropic/claude-4-sonnet-20250522',
  spawnableAgents: ["research-restaurant"],
  includeMessageHistory: true,

  toolNames: ["spawn_agents"],

  inputSchema: {
    prompt: { type: 'string', description: 'Additional context or preferences for restaurant search (optional)' },
    params: {
      type: 'object',
      properties: {
        location: {
          type: 'string',
          description: 'Location to search for restaurants (e.g., "Mission District SF", "Downtown Portland"). If omitted, location will be auto-detected.',
        },
        dietaryRestrictions: {
          type: 'array',
          items: { type: 'string' },
          description: 'List of dietary restrictions (e.g., ["gluten-free", "dairy-free", "pescatarian"]). If omitted, the agent will look them up from Mem0 before falling back to a generic default.',
        }
      }
    }
  },

  spawnerPrompt: 'Spawn when you need to find nearby restaurants',

  instructionsPrompt: `
Find restaurants that meet the specified dietary restrictions within walking distance of the specified location, preferring previously vetted spots from the user's Mem0 memory before issuing fresh Exa searches.

## Step 1 — Resolve missing inputs from Mem0 (do this BEFORE asking the user)

Before any restaurant search, check whether you need to fill in missing params from the user's stored memories via the Mem0 MCP:

1. If \`params.dietaryRestrictions\` is missing or an empty array, query Mem0 with a search like "dietary restrictions" or "food allergies". If a relevant memory is found, parse the restrictions out of it and use them. ONLY ask the user if Mem0 also returns nothing.
2. If \`params.location\` is missing, query Mem0 with a search like "where do I live" or "home address" or "current location". If a memory is found, use it as the location. ONLY ask the user if Mem0 also returns nothing.

The dietary restrictions (after resolution) are an array of strings (e.g., ["gluten-free", "dairy-free", "pescatarian"]).

## Step 2 — Mem0-first restaurant lookup (vetted spots)

Before calling Exa, search Mem0 for previously-vetted restaurants matching the request. Run 1–2 targeted searches such as:
- "<cuisine or dish, e.g. fish tacos> in <neighborhood>" (e.g. "fish tacos in the Mission")
- "favorite restaurants in <neighborhood>" or "<neighborhood> restaurants"

From the results, keep only restaurants that:
- Are within walking distance of \`location\` (~1 mile or 20 min walk)
- Are compatible with the resolved \`dietaryRestrictions\`

Call this set the **vetted set**. Apply this threshold to decide what to do next:

- **3+ vetted matches** → SKIP the Exa search. Use only the vetted set. Do not spawn research-restaurant for vetted spots by default; the Mem0 memory IS the prior research.
- **1–2 vetted matches** → Keep them, then run Step 3 (Exa) to discover additional candidates. Top up to 5 total.
- **0 vetted matches** → Run Step 3 (Exa) for the full set of candidates.

### Filling output bullets for vetted spots

For any vetted spot, extract the **Menu items** and **Dietary accommodations** bullets VERBATIM from the matching Mem0 memory text — do NOT invent dishes or accommodations the memory does not state. If the memory is too sparse to fill at least 2 bullets in either section, fall back to spawning research-restaurant FOR THAT SPECIFIC SPOT ONLY to enrich the bullets, then keep its `(from memory)` tag (the spot was still recalled from memory; research-restaurant just filled in detail).

## Step 3 — Exa discovery (only when needed per Step 2)

Use the Exa MCP to find candidate restaurants near \`location\` matching the dietary restrictions. Spawn a research-restaurant agent for each Exa candidate (in parallel) and pass the dietary restrictions. These results form the **new finds**.

## Step 4 — Auto-save new finds to Mem0

After Exa discovery completes (Step 3), persist a single consolidated memory summarizing the NEW finds for this neighborhood + cuisine/dish combination, using the same compact structure as your output. Examples of acceptable memory text:
- "Vetted gluten-free fish taco spots near the Mission, SF: Pancho Villa (Fish Guachinango on corn tortilla, no crema), Tacko (Baja beer-battered, hold lime crema), …"

Rules for the auto-save:
- ONLY save NEW finds discovered via Exa in this run. Do NOT re-save vetted spots from Step 2 (they're already in Mem0).
- ONLY save the durable facts: restaurant name, neighborhood, cuisine/dish category, dietary fit, ordering tip. NEVER persist transient context ("I'm at X right now", current date, the user's mood).
- ALWAYS encode the dietaryRestrictions context in the saved memory text (e.g., "Vetted gluten-free + dairy-free fish taco spots near the Mission, SF: …"). This makes retrieval accurate across different restriction sets later.
- If Step 3 returned zero new finds, skip the save entirely.
- Make a single \`mem0_add\` call per run, not one per restaurant — Mem0's server-side extraction will split the consolidated text into per-restaurant facts and dedupe against existing memories. When in doubt about whether a find is already stored, INCLUDE it in the save — server-side dedupe will reconcile overlaps.

User-initiated writes still take priority: if the user explicitly says "remember", "save this", "forget that", etc., honor that immediately and follow their instruction verbatim.

Do NOT write to Mem0 about anything OTHER than restaurant findings (no inferred preferences, no transient context).

## Output format

Return at most 5 restaurants total (vetted + new finds, deduped by name). When the SAME restaurant appears in both the vetted set and the Exa results, tag it `(from memory)` and prefer the Mem0-sourced bullets.

If no candidates remain after Steps 2–3 (zero vetted AND zero Exa), do NOT invent restaurants. Tell the user honestly that nothing nearby matches the criteria and suggest broadening (wider radius, fewer restrictions, or a different dish).

Format each restaurant CONCISELY using exactly this structure (and nothing else):

**<Restaurant Name>** _(from memory)_ — <distance from user, e.g. "0.4 mi" or "~8 min walk">
- Menu items:
  - <specific dish 1 that meets the dietary restrictions>
  - <specific dish 2 that meets the dietary restrictions>
  - <specific dish 3 that meets the dietary restrictions>
- Dietary accommodations:
  - <accommodation 1, e.g. "dedicated gluten-free fryer">
  - <accommodation 2, e.g. "tamari available on request">
  - <accommodation 3, e.g. "100% dairy-free kitchen">

The italicized tag immediately after the restaurant name MUST be one of:
- _(from memory)_ — for vetted spots from Step 2
- _(new find)_ — for restaurants discovered via Exa in Step 3

Use 2–5 nested bullets per section (pick the strongest 2–5; don't pad). Each bullet should be a single short phrase — no full sentences and no semicolons stacking multiple items into one bullet. Do NOT include "Perfect for", "Bonus", "Highlights", ratings, hours, addresses, ⭐ icons, or any other sections. Keep it tight.

At the very end of the output (after the last restaurant), if Step 4 wrote any new finds to Mem0, add a single italicized line:

_Saved <N> new find(s) to your Mem0 memory for next time._

Omit that line entirely if no new finds were saved.
`,

  // @ts-ignore
  "mcpServers": {
    "exa": {
      "type": "http",
      // $EXA_API_KEY is a literal placeholder — Codebuff resolves it from the
      // END USER's local environment at agent runtime. Do NOT use a JS template
      // literal like `${env.EXA_API_KEY}` here, or the publisher's key gets
      // baked into the published agent artifact.
      "url": "https://mcp.exa.ai/mcp?$EXA_API_KEY",
    },
    "mem0": {
      "type": "http",
      "url": "https://mcp.mem0.ai/mcp/",
      "headers": {
        // Same rule — literal $MEM0_API_KEY placeholder, resolved on the user's machine.
        "Authorization": "Token $MEM0_API_KEY",
      },
    },
  }
}

export default definition
