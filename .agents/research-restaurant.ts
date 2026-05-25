import type { AgentDefinition } from './types/agent-definition'

const definition: AgentDefinition = {
  id: 'research-restaurant',
  displayName: 'Research Restaurant',
  publisher: "tanzeela",

  model: 'x-ai/grok-4.3',
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
If the restaurant has suitable menu items, provide the result in the following format:
1. Restaurant name
2. Specific menu items that meet ALL of the dietary restrictions from params.dietaryRestrictions
3. 10-20 word description of the vibe of the restaurant
4. Clickable URL link to menu of the restaurant, not just the restaurant's website.
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
