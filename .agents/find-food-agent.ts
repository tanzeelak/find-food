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

const definition: AgentDefinition = {
  id: 'find-food-agent',
  displayName: 'Find Food Agent',

  model: 'anthropic/claude-4-sonnet-20250522',
  spawnableAgents: [],

  // Check out .agents/types/tools.ts for more information on the tools you can include.
  toolNames: [],

  spawnerPrompt: 'Spawn when you need to find a nearby restaurant',

  instructionsPrompt: `
Use the Exa MCP to help me find gluten-free, dairy-free, pescatarian restaurants in the Lower Haight area of San Francisco. For each restaurant, please provide:
1. Restaurant name
2. Specific menu items that meet ALL three dietary restrictions (gluten-free, dairy-free, pescatarian)
3. 10-20 word description of the vibe of the restaurant
4. Clickable URL link to menu of the restaurant, not just the restaurant's website.
`,

  // @ts-ignore
  "mcpServers": {
    "exa": {
      "type": "http",
      "url": 'https://mcp.exa.ai/mcp?exaApiKey=7d543fc7-49ba-48eb-9ca6-3c7f1216008d',
    },
    // "yelp_agent": {
    //   "command": "uv",
    //   "args": [
    //     "--directory",
    //     ".",
    //     "run",
    //     "mcp-yelp-agent"
    //   ],
    //   "env": {
    //     "YELP_API_KEY": "<YOUR_YELP_FUSION_API_KEY>"
    //   }
    // }
  }


  // Add more fields here to customize your agent further:
  // - system prompt
  // - input/output schema
  // - handleSteps

  // Check out the examples in .agents/examples for more ideas!
}

export default definition
