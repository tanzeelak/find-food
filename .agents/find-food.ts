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
For each candidate restaurant, spawn a research-restaurant agent and pass the dietary restrictions to it. Show the results of all of these agents.
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
