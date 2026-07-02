import { createTool } from "@mastra/core/tools";
import { researchRestaurantAgent } from "../agents/research-restaurant.js";
import { researchInputSchema, researchResultSchema } from "../schemas.js";

function joinOrNone(values: string[]): string {
  return values.length === 0 ? "none specified" : values.join(", ");
}

export const researchRestaurantTool = createTool({
  id: "research-restaurant",
  description:
    "Performs bounded web research on ONE restaurant and returns structured, source-backed menu items that satisfy the given dietary restrictions. Call once per candidate restaurant.",
  inputSchema: researchInputSchema,
  outputSchema: researchResultSchema,
  execute: async (input) => {
    const restaurantName = input.restaurantName;
    const dietaryRestrictions = input.dietaryRestrictions ?? [];

    const fallback = (notes: string) =>
      researchResultSchema.parse({ restaurantName, hasSuitableItems: false, notes });

    try {
      // Phase 1: tool-using research that returns free-form findings.
      const research = await researchRestaurantAgent.generate(
        `Research this restaurant and report which menu items match the food query and satisfy every dietary restriction. Cite the source URLs you used.

Restaurant: ${restaurantName}
Food query: ${input.foodQuery}
Location: ${input.location}
Dietary restrictions: ${joinOrNone(dietaryRestrictions)}`
      );

      // Phase 2: tool-free structuring pass over the findings text.
      const structured = await researchRestaurantAgent.generate(
        `Convert the following restaurant research into the required structured result for "${restaurantName}". Only include menu items the research supports as satisfying every dietary restriction. If none qualify, set hasSuitableItems to false and return an empty menuItems array.

Research:
${research.text}`,
        {
          activeTools: [],
          structuredOutput: { schema: researchResultSchema, jsonPromptInjection: true }
        }
      );

      const parsed = researchResultSchema.safeParse(structured.object);
      return parsed.success ? parsed.data : fallback("Structuring pass returned no valid result.");
    } catch (error) {
      return fallback(`Research failed: ${error instanceof Error ? error.message : String(error)}`);
    }
  }
});
