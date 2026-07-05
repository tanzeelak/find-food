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

      // Phase 2: tool-free structuring pass — model returns raw JSON, we validate with Zod.
      // Avoids Mastra's internal structured output validation throwing on Zod v4 when
      // the model returns nothing (e.g. empty research result).
      const structured = await researchRestaurantAgent.generate(
        `Convert the following restaurant research into a JSON object for "${restaurantName}".
Return ONLY a raw JSON object — no markdown, no code fences, no commentary.

Required shape:
{
  "restaurantName": string,
  "hasSuitableItems": boolean,
  "menuItems": [{ "name": string, "price": string, "whyItFits": string, "caveats": string[] }],
  "dietaryAccommodations": string[],
  "menuUrl": string,
  "sourceUrls": string[],
  "confidence": "low" | "medium" | "high",
  "notes": string
}

Only include menu items the research supports as satisfying every dietary restriction. If none qualify, set hasSuitableItems to false and menuItems to [].

Research:
${research.text}`,
        { activeTools: [] }
      );

      try {
        const cleaned = structured.text
          .replace(/^```(?:json)?\n?/, "")
          .replace(/\n?```$/, "")
          .trim();
        const parsed = researchResultSchema.safeParse(JSON.parse(cleaned));
        return parsed.success ? parsed.data : fallback("Structuring pass returned invalid shape.");
      } catch {
        return fallback("Structuring pass returned non-JSON output.");
      }
    } catch (error) {
      return fallback(`Research failed: ${error instanceof Error ? error.message : String(error)}`);
    }
  }
});
