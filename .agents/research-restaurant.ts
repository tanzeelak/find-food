import type { AgentDefinition } from './types/agent-definition'

const definition: AgentDefinition = {
  id: 'research-restaurant',
  displayName: 'Research Restaurant',
  publisher: "tanzeela",

  model: 'anthropic/claude-4-sonnet-20250522',
  spawnableAgents: [],
  includeMessageHistory: true,
  // Check out .agents/types/tools.ts for more information on the tools you can include.
  toolNames: [],

  inputSchema: {
    prompt: { type: 'string', description: 'Restaurant name to research' },
    params: {
      type: 'object',
      properties: {
        dietaryRestrictions: {
          type: 'array',
          items: { type: 'string' },
          description: 'List of dietary restrictions to check for (e.g., ["gluten-free", "dairy-free", "pescatarian"])',
          default: ["gluten-free", "dairy-free", "pescatarian"]
        }
      }
    }
  },

  spawnerPrompt: 'Provide a restaurant to research',

  instructionsPrompt: `
Use the Exa MCP to figure out if this restaurant has menu items that meet the specified dietary restrictions.
The dietary restrictions are provided in params.dietaryRestrictions as an array of strings.

## Exa MCP tool schemas (IMPORTANT — calls with empty payloads will fail)

You have two Exa MCP tools available. Each tool requires specific parameters; calling them with \`{}\` will fail with a Zod validation error.

- \`exa__web_search_exa\` — required parameters:
  - \`query\` (string, REQUIRED): the search query, e.g. "<restaurant name> menu gluten free" or "<restaurant name> dietary restrictions".
  - \`numResults\` (number, optional, default 5): how many results to return; 3–5 is plenty for restaurant research.

- \`exa__web_fetch_exa\` — required parameters:
  - \`urls\` (string[], REQUIRED): an array of one or more full http(s) URLs to fetch (e.g. the restaurant's menu page). Even for a single URL, pass it as a one-element array.

Do NOT call either tool with an empty object. Always construct \`query\` from the restaurant name + dietary keywords, and always pass concrete URLs (taken from your search results) to \`exa__web_fetch_exa\`.

## Recommended workflow

1. Call \`exa__web_search_exa\` with a query that joins the restaurant name with the dietary keywords using spaces, plus the word "menu". Concrete examples:
   - dietaryRestrictions = ["gluten-free", "dairy-free"] for "La Taqueria" → \`query: "La Taqueria menu gluten-free dairy-free"\`
   - dietaryRestrictions = ["tofu-free"] for "El Farolito" → \`query: "El Farolito menu tofu-free"\`
   Use \`numResults: 5\`.
2. Pick the 1–2 most menu-relevant URLs from the results (prefer the restaurant's own menu page, then reputable reviews / allergen guides).
3. Call \`exa__web_fetch_exa\` once with those URLs in a single \`urls\` array, e.g. \`{ urls: ["https://lataqueriasf.com/menu"] }\`.
4. From the fetched text, extract specific menu items that meet ALL dietary restrictions. Quote dish names verbatim from the source — do NOT invent dishes.

## Output format

If the restaurant has suitable menu items, provide the result in the following format:
1. Restaurant name
2. Specific menu items that meet ALL of the dietary restrictions from params.dietaryRestrictions
3. 10-20 word description of the vibe of the restaurant
4. Clickable URL link to menu of the restaurant, not just the restaurant's website.

If the restaurant has NO suitable items after a real search, say so plainly — do not fabricate dishes.
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
  }
}

export default definition
