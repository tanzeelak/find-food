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
        `You must follow these steps in order — do not skip any:

STEP 1: Call exa_web_search_exa with the query "${restaurantName} ${input.location} menu" to find the restaurant's website or a dedicated menu page.
STEP 2: From the search results, identify the best URL — prefer the restaurant's own domain or food media (Eater, Infatuation, SF Chronicle). Then call exa_web_fetch_exa on that URL to read the actual page content.
STEP 3: Extract every dish name and price from the fetched page. If the page had no dish names, call exa_web_search_exa again with "${restaurantName} full menu" and fetch the next best URL.
STEP 4: Report which dishes satisfy ALL of the dietary restrictions below. Quote dish names exactly as they appear on the menu.

You MUST call exa_web_fetch_exa at least once. Do not draw conclusions from search snippets alone.

Restaurant: ${restaurantName}
Food query: ${input.foodQuery}
Location: ${input.location}
Dietary restrictions: ${joinOrNone(dietaryRestrictions)}

Cite every URL you fetched.`
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
