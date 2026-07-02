import { Agent } from "@mastra/core/agent";
import { orchestratorModel } from "../model.js";
import { findFoodInstructions } from "../prompts.js";
import { exaSearchTools } from "../mcp/exa.js";
import { memory } from "../memory/index.js";
import { mem0RememberTool, mem0MemorizeTool } from "../memory/mem0.js";
import { researchRestaurantTool } from "../tools/research-restaurant.js";

export const findFoodAgent = new Agent({
  id: "find-food",
  name: "Find Food",
  description: "Conversational orchestrator that finds dietary-compatible menu items near a location.",
  instructions: findFoodInstructions,
  model: orchestratorModel,
  memory,
  tools: async () => ({
    ...(await exaSearchTools()),
    researchRestaurant: researchRestaurantTool,
    mem0Remember: mem0RememberTool,
    mem0Memorize: mem0MemorizeTool
  })
});
