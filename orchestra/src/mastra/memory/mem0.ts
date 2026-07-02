import { MemoryClient, type Memory } from "mem0ai";
import { createTool } from "@mastra/core/tools";
import { z } from "zod";
import { getEnv, requireEnv } from "../env.js";

// Mem0 is wired in alongside Mastra's built-in working memory so the two
// approaches can be compared side by side. Mastra working memory keeps a
// single resource-scoped profile template; Mem0 stores semantically-searchable
// memories per user. Both are driven by the same durable facts the agent
// decides to persist (see prompts.ts).

let client: MemoryClient | undefined;

function mem0Client(): MemoryClient {
  if (!client) {
    client = new MemoryClient({ apiKey: requireEnv("MEM0_API_KEY") });
  }
  return client;
}

// Scope Mem0 by the same id Mastra uses for working memory (resourceId), so a
// comparison is apples-to-apples. The id is passed per-turn via requestContext
// (see run.ts); fall back to a stable default for the chatRoute path.
const DEFAULT_USER_ID = getEnv("MEM0_USER_ID", "find-food-user");

function resolveUserId(requestContext?: { get(key: string): unknown }): string {
  const fromContext = requestContext?.get("resourceId");
  return typeof fromContext === "string" && fromContext !== "" ? fromContext : DEFAULT_USER_ID;
}

export const mem0RememberTool = createTool({
  id: "mem0-remember",
  description:
    "Search the user's Mem0 long-term memory for durable facts (dietary restrictions, allergies, usual location, food likes/dislikes, previously vetted restaurants). Call this before asking the user for context you may already know.",
  inputSchema: z.object({
    query: z.string().describe("What to look up, e.g. 'dietary restrictions and usual location'.")
  }),
  outputSchema: z.object({
    memories: z.array(z.string())
  }),
  execute: async (input, ctx) => {
    const userId = resolveUserId(ctx.requestContext);
    try {
      const response = await mem0Client().search(input.query, {
        filters: { user_id: userId },
        topK: 10
      });
      const memories = (response.results ?? [])
        .map((entry: Memory) => entry.memory)
        .filter((memory): memory is string => typeof memory === "string" && memory.length > 0);
      return { memories };
    } catch (error) {
      ctx.observe.log("warn", "mem0 search failed", {
        error: error instanceof Error ? error.message : String(error)
      });
      return { memories: [] };
    }
  }
});

export const mem0MemorizeTool = createTool({
  id: "mem0-memorize",
  description:
    "Save a durable fact about the user to Mem0 long-term memory (dietary restrictions, allergies, usual location, strong food likes/dislikes, or a vetted restaurant finding). Do NOT save transient context like a one-off craving or a single-search location.",
  inputSchema: z.object({
    statement: z.string().describe("A single durable fact to remember, phrased as a standalone sentence.")
  }),
  outputSchema: z.object({
    success: z.boolean()
  }),
  execute: async (input, ctx) => {
    const userId = resolveUserId(ctx.requestContext);
    try {
      await mem0Client().add([{ role: "user", content: input.statement }], { userId });
      return { success: true };
    } catch (error) {
      ctx.observe.log("warn", "mem0 add failed", {
        error: error instanceof Error ? error.message : String(error)
      });
      return { success: false };
    }
  }
});
