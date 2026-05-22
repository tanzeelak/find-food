import type { AgentDefinition } from './types/agent-definition'
import { env } from 'process'

const definition: AgentDefinition = {
  id: 'find-food',
  displayName: 'Find Food',
  publisher: "tanzeela",

  model: 'anthropic/claude-4-sonnet-20250522',
  spawnableAgents: ["research-restaurant"],
  includeMessageHistory: true,

  toolNames: ["spawn_agents", "run_terminal_command", "add_message"],

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
Use the Exa MCP to help me find restaurants that meet the specified dietary restrictions within 1 mile of the specified location.
The dietary restrictions are provided in params.dietaryRestrictions as an array of strings (e.g., ["gluten-free", "dairy-free", "pescatarian"]).
For each candidate restaurant, spawn a research-restaurant agent and pass the dietary restrictions to it. Show the results of all of these agents.
`,

  handleSteps: function* ({ params, logger }) {
    let location = (params as any)?.location as string | undefined

    if (!location) {
      logger.info('No location provided — detecting via IP geolocation')
      const { toolResult } = yield {
        toolName: 'run_terminal_command',
        input: { command: 'curl -s https://ipapi.co/json/', timeout_seconds: 10 },
      }

      if (toolResult?.[0]?.type === 'json') {
        try {
          const cmdOutput = toolResult[0].value as any
          const geo = JSON.parse(cmdOutput.stdout ?? '')
          if (geo.city) {
            location = [geo.city, geo.region_code].filter(Boolean).join(', ')
            logger.info(`Detected location: ${location}`)
          }
        } catch {
          // fall through — location stays undefined
        }
      }
    }

    const restrictions = (params as any)?.dietaryRestrictions ?? []

    yield {
      toolName: 'add_message',
      input: {
        role: 'user',
        content: location
          ? `Find restaurants near "${location}" with dietary restrictions: ${JSON.stringify(restrictions)}`
          : `Could not auto-detect location. Dietary restrictions: ${JSON.stringify(restrictions)}. Please ask the user for their location before searching.`,
      },
    }

    yield 'STEP_ALL'
  },

  // @ts-ignore
  "mcpServers": {
    "exa": {
      "type": "http",
      "url": `https://mcp.exa.ai/mcp?${env.EXA_API_KEY}`,
    },
  }
}

export default definition
