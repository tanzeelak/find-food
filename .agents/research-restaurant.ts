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
  id: 'research-restaurant',
  displayName: 'Research Restaurant',
  publisher: "tanzeela",

  model: 'x-ai/grok-4-fast:free',
  spawnableAgents: [],
  includeMessageHistory: true,
  // Check out .agents/types/tools.ts for more information on the tools you can include.
  toolNames: [],

  spawnerPrompt: 'Provide a restaurant to research',

  instructionsPrompt: `
Use the Exa MCP to figure out if this restaurant has menu items that meet my dietary restrictions
If so, provide the result in following ormat:
1. Restaurant name
2. Specific menu items that meet ALL three dietary restrictions (gluten-free, dairy-free, pescatarian)
3. 10-20 word description of the vibe of the restaurant
4. Clickable URL link to menu of the restaurant, not just the restaurant's website.
`,

  // @ts-ignore
  "mcpServers": {
    "exa": {
      "type": "http",
      "url": 'https://mcp.exa.ai/mcp',
    },
  }


  // Add more fields here to customize your agent further:
  // - system prompt
  // - input/output schema
  // - handleSteps

  // Check out the examples in .agents/examples for more ideas!
}

export default definition
