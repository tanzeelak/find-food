package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"find-restaurants/internal/config"
	"find-restaurants/internal/exa"
	"find-restaurants/internal/llm"
)

func TestWorkflowNeedsLocation(t *testing.T) {
	workflow := testWorkflow()

	response, err := workflow.Run(context.Background(), FindFoodRequest{
		Message:             "I want gluten-free fish tacos near me",
		DietaryRestrictions: []string{"gluten-free"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if response.Status != "needs_input" {
		t.Fatalf("expected needs_input, got %s", response.Status)
	}
	if response.FollowUpQuestion == nil {
		t.Fatal("expected follow-up question")
	}
	if response.ConversationID == "" {
		t.Fatal("expected conversation id")
	}
}

func TestWorkflowCoreAgentHandlesVagueInput(t *testing.T) {
	workflow := testWorkflow()

	response, err := workflow.Run(context.Background(), FindFoodRequest{
		Message: "I want this",
	})
	if err != nil {
		t.Fatal(err)
	}

	if response.Status != "needs_input" {
		t.Fatalf("expected needs_input, got %s", response.Status)
	}
	if response.FollowUpQuestion == nil || *response.FollowUpQuestion == "" {
		t.Fatal("expected follow-up question")
	}
	if response.ConversationID == "" {
		t.Fatal("expected conversation id")
	}
	if len(response.Warnings) != 3 {
		t.Fatalf("expected three missing-field warnings, got %#v", response.Warnings)
	}
}

func TestWorkflowConversationContinuesAfterFollowUp(t *testing.T) {
	workflow := testWorkflow()

	first, err := workflow.Run(context.Background(), FindFoodRequest{
		Message:             "I want gluten-free fish tacos near me",
		DietaryRestrictions: []string{"gluten-free"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "needs_input" {
		t.Fatalf("expected first response needs_input, got %s", first.Status)
	}
	if first.ConversationID == "" {
		t.Fatal("expected conversation id")
	}

	second, err := workflow.Run(context.Background(), FindFoodRequest{
		ConversationID: first.ConversationID,
		Message:        "Mission District SF",
	})
	if err != nil {
		t.Fatal(err)
	}

	if second.Status != "complete" {
		t.Fatalf("expected complete after follow-up, got %s", second.Status)
	}
	if second.ConversationID != first.ConversationID {
		t.Fatalf("expected same conversation id, got %s", second.ConversationID)
	}
	if second.Metadata == nil || second.Metadata.FoodQuery != "fish tacos" || second.Metadata.Location != "Mission District SF" {
		t.Fatalf("expected merged conversation metadata, got %#v", second.Metadata)
	}
}

func TestWorkflowComplete(t *testing.T) {
	workflow := testWorkflow()

	response, err := workflow.Run(context.Background(), FindFoodRequest{
		Message:             "I want gluten-free fish tacos near me",
		Location:            "Mission District SF",
		DietaryRestrictions: []string{"gluten-free"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if response.Status != "complete" {
		t.Fatalf("expected complete, got %s", response.Status)
	}
	if len(response.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(response.Items))
	}
	if response.Items[0].RestaurantSource != "new_find" {
		t.Fatalf("expected new_find source, got %s", response.Items[0].RestaurantSource)
	}
	if response.Items[0].RestaurantName != "Mission Tacos" {
		t.Fatalf("expected restaurant name on item, got %s", response.Items[0].RestaurantName)
	}
	if response.Items[0].Name != "Fish tacos on corn tortillas" {
		t.Fatalf("expected item name, got %s", response.Items[0].Name)
	}
	if response.Metadata == nil || response.Metadata.Location != "Mission District SF" {
		t.Fatalf("expected metadata location, got %#v", response.Metadata)
	}
}

func TestWorkflowFlattensMultipleItems(t *testing.T) {
	items := flattenFoodItems([]RestaurantResult{
		{
			Name:                  "Mission Tacos",
			Source:                "new_find",
			DietaryAccommodations: []string{"Corn tortillas available"},
			MenuURL:               "https://example.com/menu",
			SourceURLs:            []string{"https://example.com/menu"},
			Confidence:            "high",
			MenuItems: []MenuItem{
				{Name: "Fish Taco", WhyItFits: "Listed gluten-free"},
				{Name: "Salmon Taco", WhyItFits: "Listed gluten-free"},
			},
		},
	})

	if len(items) != 2 {
		t.Fatalf("expected two item results, got %d", len(items))
	}
	if items[1].RestaurantName != "Mission Tacos" || items[1].Name != "Salmon Taco" {
		t.Fatalf("unexpected flattened item: %#v", items[1])
	}
}

func TestWorkflowFiltersUnsupportedDietaryCaveats(t *testing.T) {
	items := cleanMenuItems([]MenuItem{
		{
			Name:      "Baja Fish Taco",
			WhyItFits: "Beer battered fish in corn tortillas",
			Caveats:   []string{"Beer batter likely contains gluten"},
		},
		{
			Name:      "Fish Taco on Corn Tortilla",
			WhyItFits: "Listed as gluten-free fish taco",
			Caveats:   []string{"Ask about cross-contamination"},
		},
		{
			Name:      "Fish or Shrimp Tacos",
			WhyItFits: "Would be gluten-free with corn tortillas",
			Caveats:   []string{"Verify tortillas are corn-based for gluten-free option"},
		},
	}, []string{"gluten-free"})

	if len(items) != 1 {
		t.Fatalf("expected one supported item, got %d", len(items))
	}
	if items[0].Name != "Fish Taco on Corn Tortilla" {
		t.Fatalf("unexpected item kept: %s", items[0].Name)
	}
}

func TestChooseFollowUpQuestionCoversMissingFields(t *testing.T) {
	question := chooseFollowUpQuestion(AgentDecision{
		FollowUpQuestion: "What food are you looking for, and where should I search?",
	}, []string{"foodQuery", "location", "dietaryRestrictions"})

	expected := "What food are you looking for, where should I search, and what dietary restrictions should I apply?"
	if question != expected {
		t.Fatalf("expected guardrail question %q, got %q", expected, question)
	}
}

func testWorkflow() *Workflow {
	return NewWorkflow(fakeLLM{}, fakeSearch{}, config.WorkflowConfig{
		MaxCandidates:       3,
		MaxResults:          2,
		ResearchConcurrency: 2,
	})
}

type fakeLLM struct{}

func (fakeLLM) ChatJSON(ctx context.Context, req llm.ChatJSONRequest, out any) error {
	var payload any
	switch req.Operation {
	case "core_food_agent":
		if strings.Contains(req.User, `"message": "I want this"`) {
			payload = AgentDecision{
				Action:           "ask_followup",
				FollowUpQuestion: "What food should I look for, where should I search, and what dietary restrictions should I apply?",
				MissingFields:    []string{"foodQuery", "location", "dietaryRestrictions"},
			}
		} else if strings.Contains(req.User, `"message": "Mission District SF"`) && strings.Contains(req.User, `"foodQuery": "fish tacos"`) {
			payload = AgentDecision{
				Action: "call_find_menu_items",
				ToolRequest: FindMenuItemsToolRequest{
					FoodQuery:           "fish tacos",
					Location:            "Mission District SF",
					LocationIntent:      "explicit",
					DietaryRestrictions: []string{"gluten-free"},
				},
			}
		} else if strings.Contains(req.User, `"location": "Mission District SF"`) {
			payload = AgentDecision{
				Action: "call_find_menu_items",
				ToolRequest: FindMenuItemsToolRequest{
					FoodQuery:           "fish tacos",
					Location:            "Mission District SF",
					LocationIntent:      "explicit",
					DietaryRestrictions: []string{"gluten-free"},
				},
			}
		} else {
			payload = AgentDecision{
				Action:           "ask_followup",
				FollowUpQuestion: "Where should I search?",
				MissingFields:    []string{"location"},
				KnownFields: AgentKnownFields{
					FoodQuery:           "fish tacos",
					LocationIntent:      "near_me",
					DietaryRestrictions: []string{"gluten-free"},
				},
			}
		}
	case "extract_candidates":
		payload = struct {
			Candidates []Candidate `json:"candidates"`
		}{
			Candidates: []Candidate{
				{Name: "Mission Tacos", URL: "https://example.com/mission-tacos"},
			},
		}
	case "extract_menu_items":
		payload = struct {
			RestaurantName        string     `json:"restaurantName"`
			HasSuitableItems      bool       `json:"hasSuitableItems"`
			MenuItems             []MenuItem `json:"menuItems"`
			DietaryAccommodations []string   `json:"dietaryAccommodations"`
			MenuURL               string     `json:"menuUrl"`
			SourceURLs            []string   `json:"sourceUrls"`
			Confidence            string     `json:"confidence"`
		}{
			RestaurantName:   "Mission Tacos",
			HasSuitableItems: true,
			MenuItems: []MenuItem{
				{Name: "Fish tacos on corn tortillas", WhyItFits: "Corn tortillas and no listed gluten ingredients"},
			},
			DietaryAccommodations: []string{"Ask for no crema"},
			MenuURL:               "https://example.com/menu",
			SourceURLs:            []string{"https://example.com/menu"},
			Confidence:            "medium",
		}
	default:
		payload = map[string]any{}
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, out)
}

type fakeSearch struct{}

func (fakeSearch) SearchRestaurantCandidates(ctx context.Context, req exa.RestaurantSearchRequest, numResults int) (exa.SearchResponse, error) {
	return exa.SearchResponse{
		Query:     "mock discovery query",
		RequestID: "mock",
		Results: []exa.Result{
			{
				Title:      "Mission Tacos Menu",
				URL:        "https://example.com/mission-tacos",
				Highlights: []string{"gluten-free fish tacos"},
			},
		},
	}, nil
}

func (fakeSearch) SearchMenuSources(ctx context.Context, req exa.MenuSearchRequest, numResults int) (exa.SearchResponse, error) {
	return exa.SearchResponse{
		Query:     "mock menu query",
		RequestID: "mock",
		Results: []exa.Result{
			{
				Title: "Mission Tacos Menu",
				URL:   "https://example.com/menu",
				Text:  "Fish tacos on corn tortillas. Salsa verde. Ask for no crema.",
			},
		},
	}, nil
}
