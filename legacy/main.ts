import { CodebuffClient } from '@codebuff/sdk'

async function main() {
  const client = new CodebuffClient({
    // You need to pass in your own API key here.
    // Get one here: https://www.codebuff.com/api-keys
    apiKey: process.env.CODEBUFF_API_KEY,
  })

  // First run
  const run1 = await client.run({
    agent: 'base',
    prompt: 'Create a simple calculator class',
    handleEvent: (event) => {
      // All events that happen during the run: agent start/finish, tool calls/results, text responses, errors.
      console.log('Event', event)
    },
  })

  // Continue the same session with a follow-up
  const run2 = await client.run({
    agent: 'base',
    prompt: 'Add unit tests for the calculator',
    previousRun: run1, // <-- this is where your next run differs from the previous run
    handleEvent: (event) => {
      console.dir(event, { depth: null })
    },
  })
}

main()