import type { AgentDefinition } from './types/agent-definition'
import { env } from 'process'

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
          description: 'List of dietary restrictions (e.g., ["gluten-free", "dairy-free", "pescatarian"])',
          default: ["gluten-free", "dairy-free", "pescatarian"]
        }
      }
    }
  },

  spawnerPrompt: 'Spawn when you need to find nearby restaurants',

  instructionsPrompt: `
Use the Exa MCP to help find restaurants that meet the specified dietary restrictions within 1 mile of the specified location.
The dietary restrictions are provided in params.dietaryRestrictions as an array of strings (e.g., ["gluten-free", "dairy-free", "pescatarian"]).
If params.location is not provided, ask the user for their location before searching.
Return at most 5 restaurants in your final output. Pick the 5 best candidates that match the dietary restrictions and proximity criteria.
For each of the (up to 5) candidate restaurants, spawn a research-restaurant agent and pass the dietary restrictions to it.

Format each restaurant result CONCISELY using exactly this structure (and nothing else):

**<Restaurant Name>** — <distance from user, e.g. "0.4 mi" or "~8 min walk">
- Menu items: <comma-separated list of specific dishes that meet the dietary restrictions>
- Dietary accommodations: <short summary of how the restaurant accommodates the restrictions, e.g. "dedicated GF fryer; tamari available; 100% dairy-free">

Do NOT include "Perfect for", "Bonus", "Highlights", ratings, hours, addresses, ⭐ icons, or any other sections. Keep it tight.
`,

  // @ts-ignore
  "mcpServers": {
    "exa": {
      "type": "http",
      "url": `https://mcp.exa.ai/mcp?${env.EXA_API_KEY}`,
    },
  }
}

export default definition
