import type { AgentDefinition } from './types/agent-definition'

const definition: AgentDefinition = {
  id: 'simple-code-reviewer',
  displayName: 'Simple Code Reviewer',
  model: 'anthropic/claude-4-sonnet-20250522',

  // Tools this agent can use
  toolNames: [
    'read_files',
    'run_terminal_command',
    'code_search',
    'spawn_agents',
  ],

  // Other agents this agent can spawn
  // Browse https://www.codebuff.com/store to see available agents
  spawnableAgents: ['codebuff/file-explorer@0.0.2'],

  // When should other agents spawn this one?
  spawnerPrompt: 'Spawn when you need to review local code changes',

  // System prompt defines the agent's identity
  systemPrompt: `You are an expert software developer specializing in code review.
Your job is to review code changes and provide helpful, constructive feedback.`,

  // Instructions for what the agent should do
  instructionsPrompt: `Review code changes by following these steps:
1. Use git diff to see what changed
2. Read the modified files to understand the context
3. Look for potential issues: bugs, security problems, style violations
4. Suggest specific improvements with examples
5. Highlight what was done well`,
}

export default definition
