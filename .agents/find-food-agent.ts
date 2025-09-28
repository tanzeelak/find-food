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
  id: 'find-food',
  displayName: 'Find Food',
  publisher: "tanzeela",

  model: 'anthropic/claude-4-sonnet-20250522',
  spawnableAgents: ["research-restaurant"],

  // Check out .agents/types/tools.ts for more information on the tools you can include.
  toolNames: ["spawn_agents"],
  // "write_file"

  spawnerPrompt: 'Spawn when you need to find a nearby restaurants',

  instructionsPrompt: `
Use the Exa MCP to help me find gluten-free, dairy-free, pescatarian friendly restaurants in the Lower Haight area of San Francisco. 
1. For each candidate restuarant, spawn restaurant-researcher. Just show the the results of all of these agents.
`,
// 2. Output to a csv file with the current prompt and the results

  // @ts-ignore
  "mcpServers": {
    "exa": {
      "type": "http",
      "url": 'https://mcp.exa.ai/mcp?exaApiKey=7d543fc7-49ba-48eb-9ca6-3c7f1216008d',
    },
  }


  // Add more fields here to customize your agent further:
  // - system prompt
  // - input/output schema
  // - handleSteps

  // Check out the examples in .agents/examples for more ideas!
}

export default definition
