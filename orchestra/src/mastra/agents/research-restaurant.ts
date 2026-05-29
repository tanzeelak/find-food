import { Agent } from "@mastra/core/agent";
import { researchModel } from "../model.js";
import { researchRestaurantInstructions } from "../prompts.js";
import { exaTools } from "../mcp/exa.js";

export const researchRestaurantAgent = new Agent({
  id: "research-restaurant",
  name: "Research Restaurant",
  description: "Bounded agent that researches a single restaurant's menu against dietary restrictions.",
  instructions: researchRestaurantInstructions,
  model: researchModel,
  tools: async () => exaTools()
});
