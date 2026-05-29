package agent

import (
	"encoding/json"
	"fmt"
)

const IntentSystemPrompt = `You parse natural-language restaurant requests into strict JSON.
Return only a JSON object. Do not include markdown.`

func BuildIntentUserPrompt(request FindFoodRequest) string {
	payload, _ := json.MarshalIndent(request, "", "  ")
	return fmt.Sprintf(`Parse this request for a restaurant-finding backend.

Request JSON:
%s

Return this JSON shape:
{
  "foodQuery": "specific food, cuisine, meal, or craving",
  "locationIntent": "explicit|near_me|unspecified",
  "location": "explicit location if present, otherwise empty string",
  "dietaryRestrictions": ["restriction from prompt only"],
  "preferences": ["other stable preferences from prompt"],
  "missingFields": ["foodQuery" or "location" only when truly missing"],
  "followUpQuestion": "short question if needed, otherwise null"
}

Rules:
- Treat "near me" as locationIntent "near_me" and location empty unless the request has a concrete place.
- Do not infer dietary restrictions unless they are explicitly in the request.
- A broad query like "dinner" or "something good" is acceptable as a foodQuery.`, payload)
}

const CandidateExtractionSystemPrompt = `You extract restaurant candidates from search results for a food search product.
Return only a JSON object. Do not include markdown.`

func BuildCandidateExtractionUserPrompt(intent Intent, location string, results []SearchResultForPrompt) string {
	payload, _ := json.MarshalIndent(results, "", "  ")
	return fmt.Sprintf(`Extract up to 8 restaurant candidates that are relevant to the food query and location.

Food query: %s
Location: %s
Dietary restrictions: %s

Search results:
%s

Return this JSON shape:
{
  "candidates": [
    {
      "name": "Restaurant name",
      "neighborhood": "neighborhood or empty string",
      "url": "best source URL or empty string",
      "reason": "brief relevance reason"
    }
  ]
}

Rules:
- Prefer actual restaurant names over listicle/article names.
- Exclude delivery aggregators unless no better source exists.
- Do not invent restaurants that are not supported by the search results.`, intent.FoodQuery, location, joinOrNone(intent.DietaryRestrictions), payload)
}

const MenuExtractionSystemPrompt = `You inspect restaurant menu/source text for individual dietary-compatible food items.
Return only a JSON object. Do not include markdown.`

func BuildMenuExtractionUserPrompt(candidate Candidate, intent Intent, location string, sources []SearchResultForPrompt) string {
	candidatePayload, _ := json.MarshalIndent(candidate, "", "  ")
	sourcePayload, _ := json.MarshalIndent(sources, "", "  ")
	return fmt.Sprintf(`Research this restaurant and return only individual menu items that match the user's food query and dietary restrictions.

Restaurant candidate:
%s

Food query: %s
Location: %s
Dietary restrictions: %s

Source pages:
%s

Return this JSON shape:
{
  "restaurantName": "canonical restaurant name",
  "hasSuitableItems": true,
  "distanceText": "distance/walk estimate if source supports it, otherwise empty string",
  "menuItems": [
    {
      "name": "specific menu item name",
      "whyItFits": "short evidence-based reason",
      "caveats": ["short caveat if any"]
    }
  ],
  "dietaryAccommodations": ["short source-backed accommodation"],
  "menuUrl": "best menu URL",
  "sourceUrls": ["URLs used"],
  "confidence": "low|medium|high",
  "notes": "short note if evidence is limited, otherwise empty string"
}

Rules:
- Only include menu items supported by source text.
- The item must match the food query or be a close substitute.
- Return individual dishes/items, not a general restaurant recommendation.
- Each menuItems entry must stand on its own as something the user can order.
- Treat dietary restrictions as hard filters. If the source does not support that an item satisfies every restriction, do not include it.
- Caveats may mention cross-contamination or ordering instructions only after the item itself satisfies the restrictions.
- Do not include items where the caveat would be "verify this is gluten-free/dairy-free/etc." Those are not suitable Phase 1 results.
- For gluten-free requests, exclude beer-battered, flour, tempura, wheat, breaded, or flour-tortilla items unless the source explicitly says the item is gluten-free.
- Do not fabricate accommodations.
- If sources do not support suitable items, set hasSuitableItems false and return an empty menuItems array.`, candidatePayload, intent.FoodQuery, location, joinOrNone(intent.DietaryRestrictions), sourcePayload)
}

type SearchResultForPrompt struct {
	Title      string   `json:"title"`
	URL        string   `json:"url"`
	Text       string   `json:"text,omitempty"`
	Highlights []string `json:"highlights,omitempty"`
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none specified"
	}

	out := values[0]
	for _, value := range values[1:] {
		out += ", " + value
	}
	return out
}
