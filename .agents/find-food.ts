/*
 *  EDIT ME to create your own agent!
 *
 *  Change any field below, and consult the AgentDefinition type for information on all fields and their purpose.
 *
 *  Run your agent with:
 *  > codebuff --agent git-committer
 *
 *  Or, run codebuff normally, and use the '@' menu to mention your agent, and codebuff will spawn it for you.
 *
 *  Finally, you can publish your agent with 'codebuff publish your-custom-agent' so users from around the world can run it.
 */

import type { AgentDefinition } from './types/agent-definition'
import { env } from 'process'

const definition: AgentDefinition = {
  id: 'find-food',
  displayName: 'Find Food',
  publisher: "tanzeela",

  model: 'anthropic/claude-4-sonnet-20250522',
  spawnableAgents: ["research-restaurant"],
  includeMessageHistory: true,

  // Check out .agents/types/tools.ts for more information on the tools you can include.
  toolNames: ["spawn_agents"],
  // "write_file"

  inputSchema: {
    prompt: { type: 'string', description: 'Additional context or preferences for restaurant search (optional)' },
    params: {
      type: 'object',
      properties: {
        location: {
          type: 'string',
          description: 'Location to search for restaurants (e.g., "Mission District SF", "Downtown Portland")',
          default: "Mission District SF"
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
Use the Exa MCP to help me find restaurants that meet the specified dietary restrictions within 1 mile of the specified location.
The location is provided in params.location (defaults to "Mission District SF" if not specified).
The dietary restrictions are provided in params.dietaryRestrictions as an array of strings (e.g., ["gluten-free", "dairy-free", "pescatarian"]).
1. For each candidate restaurant, spawn research-restaurant agent and pass the dietary restrictions to it. Just show the results of all of these agents.
`,
// 2. Output to a csv file with the current prompt and the results

  // @ts-ignore
  "mcpServers": {
    "exa": {
      "type": "http",
      "url": `https://mcp.exa.ai/mcp?${env.EXA_API_KEY}`,
    },
  }


  // Add more fields here to customize your agent further:
  // - system prompt
  // - input/output schema
  // - handleSteps

  // Check out the examples in .agents/examples for more ideas!
}

export default definition
