package agent

import (
	"encoding/json"
	"fmt"
)

const CoreAgentSystemPrompt = `You are the core food agent for a dietary menu-item search product.
Decide whether to ask the user a follow-up question or call the backend find_menu_items tool.
Return only a JSON object. Do not include markdown.`

func BuildCoreAgentUserPrompt(request FindFoodRequest, conversation ConversationContext) string {
	requestPayload, _ := json.MarshalIndent(request, "", "  ")
	conversationPayload, _ := json.MarshalIndent(conversation, "", "  ")
	return fmt.Sprintf(`Handle this user request for a backend that finds individual menu items matching dietary restrictions.

Current request JSON:
%s

Conversation context JSON:
%s

Available backend tools:
- find_menu_items: searches for individual restaurant menu items that match a food query, location, and dietary restrictions.

Return this JSON shape:
{
  "action": "ask_followup|call_find_menu_items",
  "followUpQuestion": "one concise natural question when action is ask_followup, otherwise empty string",
  "missingFields": ["foodQuery", "location", "dietaryRestrictions"],
  "knownFields": {
    "foodQuery": "known food/dish/cuisine/craving or empty string",
    "location": "known explicit location or empty string",
    "locationIntent": "explicit|near_me|unspecified",
    "dietaryRestrictions": ["known restrictions only"],
    "preferences": ["other useful preferences"]
  },
  "toolRequest": {
    "toolName": "find_menu_items",
    "foodQuery": "complete food/dish/cuisine/craving",
    "location": "complete search location",
    "locationIntent": "explicit|near_me|unspecified",
    "dietaryRestrictions": ["complete restrictions"],
    "preferences": ["other useful preferences"]
  }
}

Rules:
- Treat current request.message as the user's latest turn.
- Use conversation context to preserve fields collected in earlier turns.
- If the latest message answers a previous follow-up question, merge that answer with knownFields from conversation context.
- The user can override earlier known fields in the latest message.
- Use action "call_find_menu_items" only when foodQuery, location, and dietaryRestrictions are all known.
- Use action "ask_followup" when anything needed is missing.
- Treat "near me" as locationIntent "near_me"; it is not a complete location unless request.location or clientLocation gives a concrete place.
- Do not infer dietary restrictions. Only use restrictions from the user or request fields.
- A broad query like "dinner", "something spicy", or "good breakfast" is acceptable as foodQuery.
- If the user says something vague like "I want this", ask what food they mean plus any other missing fields.
- Ask one compact question that covers all missing fields.
- When action is "ask_followup", fill knownFields and leave incomplete toolRequest fields empty.
- When action is "call_find_menu_items", toolRequest.toolName must be "find_menu_items", missingFields must be empty, and toolRequest must be complete.`, requestPayload, conversationPayload)
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
