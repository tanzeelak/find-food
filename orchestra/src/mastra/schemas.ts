import { z } from "zod";

export const confidenceSchema = z.enum(["low", "medium", "high"]);

export const menuItemSchema = z.object({
  name: z.string().describe("Specific orderable menu item name"),
  price: z
    .string()
    .default("")
    .describe("Price as it appears in the source, including currency symbol (e.g. \"$14\"). Empty string if unknown."),
  whyItFits: z.string().describe("Short, evidence-based reason it satisfies the restrictions"),
  caveats: z.array(z.string()).default([]).describe("Ordering tips or cross-contamination notes, if any")
});

export const researchInputSchema = z.object({
  restaurantName: z.string().describe("Name of the restaurant to research"),
  foodQuery: z.string().describe("The dish or cuisine the user is looking for"),
  location: z.string().describe("City / neighborhood the search is centered on"),
  dietaryRestrictions: z.array(z.string()).default([]).describe("Hard dietary filters to apply")
});

export const researchResultSchema = z.object({
  restaurantName: z.string(),
  hasSuitableItems: z.boolean(),
  distanceText: z.string().default(""),
  menuItems: z.array(menuItemSchema).default([]),
  dietaryAccommodations: z.array(z.string()).default([]),
  menuUrl: z.string().default(""),
  sourceUrls: z.array(z.string()).default([]),
  confidence: confidenceSchema.default("medium"),
  notes: z.string().default("")
});

export type MenuItem = z.infer<typeof menuItemSchema>;
export type ResearchInput = z.infer<typeof researchInputSchema>;
export type ResearchResult = z.infer<typeof researchResultSchema>;
