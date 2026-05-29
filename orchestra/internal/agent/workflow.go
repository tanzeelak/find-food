package agent

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"find-restaurants/internal/apperrors"
	"find-restaurants/internal/config"
	"find-restaurants/internal/exa"
	"find-restaurants/internal/llm"
)

type LLMClient interface {
	ChatJSON(ctx context.Context, req llm.ChatJSONRequest, out any) error
}

type SearchClient interface {
	SearchRestaurantCandidates(ctx context.Context, req exa.RestaurantSearchRequest, numResults int) (exa.SearchResponse, error)
	SearchMenuSources(ctx context.Context, req exa.MenuSearchRequest, numResults int) (exa.SearchResponse, error)
}

type Workflow struct {
	llm           LLMClient
	search        SearchClient
	config        config.WorkflowConfig
	conversations *conversationStore
}

func NewWorkflow(llmClient LLMClient, searchClient SearchClient, cfg config.WorkflowConfig) *Workflow {
	if cfg.MaxCandidates == 0 {
		cfg.MaxCandidates = 6
	}
	if cfg.MaxResults == 0 {
		cfg.MaxResults = 5
	}
	if cfg.ResearchConcurrency == 0 {
		cfg.ResearchConcurrency = 3
	}

	return &Workflow{
		llm:           llmClient,
		search:        searchClient,
		config:        cfg,
		conversations: newConversationStore(30 * time.Minute),
	}
}

func (w *Workflow) Run(ctx context.Context, raw FindFoodRequest) (FindFoodResponse, error) {
	start := time.Now()
	log.Printf("workflow started")

	request, err := normalizeRequest(raw)
	if err != nil {
		return FindFoodResponse{}, err
	}
	conversationID, conversation := w.loadConversation(request)
	request.ConversationID = conversationID

	log.Printf("workflow core agent started")
	decision, err := w.runCoreAgent(ctx, request, conversation)
	if err != nil {
		return FindFoodResponse{}, err
	}

	log.Printf("workflow core agent complete action=%s missing=%q duration=%s", decision.Action, decision.MissingFields, time.Since(start).Round(time.Millisecond))
	intent := intentFromAgentDecision(request, conversation, decision)
	location := resolveLocation(request, intent)
	missingFields := validateSearchIntent(intent, location)

	if decision.Action == "ask_followup" {
		missingFields = normalizeMissingFields(append(decision.MissingFields, missingFields...))
		question := chooseFollowUpQuestion(decision, missingFields)
		w.saveNeedsInputConversation(conversation, request, decision, missingFields, question)
		log.Printf("workflow needs input missing=%q duration=%s", missingFields, time.Since(start).Round(time.Millisecond))
		return FindFoodResponse{
			ConversationID:   conversationID,
			Status:           "needs_input",
			Items:            []FoodItemResult{},
			FollowUpQuestion: &question,
			Warnings:         missingWarnings(missingFields),
		}, nil
	}

	if len(missingFields) > 0 {
		question := chooseFollowUpQuestion(decision, missingFields)
		w.saveNeedsInputConversation(conversation, request, decision, missingFields, question)
		log.Printf("workflow core agent guardrail needs input missing=%q duration=%s", missingFields, time.Since(start).Round(time.Millisecond))

		return FindFoodResponse{
			ConversationID:   conversationID,
			Status:           "needs_input",
			Items:            []FoodItemResult{},
			FollowUpQuestion: &question,
			Warnings:         missingWarnings(missingFields),
		}, nil
	}

	return w.runFindMenuItemsTool(ctx, start, conversationID, conversation, request, intent, location)
}

func (w *Workflow) runFindMenuItemsTool(ctx context.Context, start time.Time, conversationID string, conversation ConversationContext, request FindFoodRequest, intent Intent, location string) (FindFoodResponse, error) {
	log.Printf("workflow tool find_menu_items started food_query=%q location=%q", intent.FoodQuery, location)
	log.Printf("workflow exa discovery started food_query=%q location=%q", intent.FoodQuery, location)
	discovery, err := w.search.SearchRestaurantCandidates(ctx, exa.RestaurantSearchRequest{
		FoodQuery:           intent.FoodQuery,
		Location:            location,
		DietaryRestrictions: intent.DietaryRestrictions,
	}, max(w.config.MaxCandidates, 8))
	if err != nil {
		return FindFoodResponse{}, err
	}
	log.Printf("workflow exa discovery complete raw_results=%d query=%q duration=%s", len(discovery.Results), discovery.Query, time.Since(start).Round(time.Millisecond))

	log.Printf("workflow candidate extraction started")
	candidates, err := w.extractCandidates(ctx, intent, location, discovery)
	if err != nil {
		return FindFoodResponse{}, err
	}
	candidates = limitCandidates(candidates, w.config.MaxCandidates)
	log.Printf("workflow candidate extraction complete candidates=%d names=%q duration=%s", len(candidates), candidateNames(candidates), time.Since(start).Round(time.Millisecond))

	log.Printf("workflow menu research started candidates=%d concurrency=%d", len(candidates), w.config.ResearchConcurrency)
	restaurants, warnings := w.researchCandidates(ctx, candidates, intent, location)
	restaurants = filterSupportedRestaurants(dedupeRestaurants(restaurants))
	items := dedupeFoodItems(flattenFoodItems(restaurants))
	if len(items) > w.config.MaxResults {
		items = items[:w.config.MaxResults]
	}
	if len(items) == 0 {
		warnings = append(warnings, "no_matching_menu_items_found")
	}
	log.Printf("workflow complete items=%d warnings=%d duration=%s", len(items), len(warnings), time.Since(start).Round(time.Millisecond))
	w.saveCompleteConversation(conversation, request, intent)

	return FindFoodResponse{
		ConversationID:   conversationID,
		Status:           "complete",
		Items:            items,
		FollowUpQuestion: nil,
		Warnings:         unique(warnings),
		Metadata: &ResponseMetadata{
			Location:            location,
			FoodQuery:           intent.FoodQuery,
			DietaryRestrictions: intent.DietaryRestrictions,
			DiscoveryQuery:      discovery.Query,
			CandidateCount:      len(candidates),
		},
	}, nil
}

func (w *Workflow) runCoreAgent(ctx context.Context, request FindFoodRequest, conversation ConversationContext) (AgentDecision, error) {
	var decision AgentDecision
	if err := w.llm.ChatJSON(ctx, llm.ChatJSONRequest{
		Operation: "core_food_agent",
		System:    CoreAgentSystemPrompt,
		User:      BuildCoreAgentUserPrompt(request, conversation),
		MaxTokens: 900,
	}, &decision); err != nil {
		return AgentDecision{}, err
	}

	decision.Action = strings.ToLower(strings.TrimSpace(decision.Action))
	if decision.Action != "call_find_menu_items" && decision.Action != "ask_followup" {
		decision.Action = "ask_followup"
	}
	decision.FollowUpQuestion = strings.TrimSpace(decision.FollowUpQuestion)
	decision.MissingFields = normalizeMissingFields(decision.MissingFields)
	decision.KnownFields.FoodQuery = strings.TrimSpace(decision.KnownFields.FoodQuery)
	decision.KnownFields.Location = strings.TrimSpace(decision.KnownFields.Location)
	decision.KnownFields.LocationIntent = normalizeLocationIntent(decision.KnownFields.LocationIntent)
	decision.KnownFields.DietaryRestrictions = unique(decision.KnownFields.DietaryRestrictions)
	decision.KnownFields.Preferences = unique(decision.KnownFields.Preferences)
	decision.ToolRequest.FoodQuery = strings.TrimSpace(decision.ToolRequest.FoodQuery)
	decision.ToolRequest.ToolName = strings.TrimSpace(decision.ToolRequest.ToolName)
	if decision.Action == "call_find_menu_items" {
		decision.ToolRequest.ToolName = "find_menu_items"
	}
	decision.ToolRequest.Location = strings.TrimSpace(decision.ToolRequest.Location)
	decision.ToolRequest.LocationIntent = normalizeLocationIntent(decision.ToolRequest.LocationIntent)
	decision.ToolRequest.DietaryRestrictions = unique(decision.ToolRequest.DietaryRestrictions)
	decision.ToolRequest.Preferences = unique(decision.ToolRequest.Preferences)

	return decision, nil
}

func (w *Workflow) extractCandidates(ctx context.Context, intent Intent, location string, discovery exa.SearchResponse) ([]Candidate, error) {
	results := promptResults(discovery.Results, 700)
	var output struct {
		Candidates []Candidate `json:"candidates"`
	}

	if err := w.llm.ChatJSON(ctx, llm.ChatJSONRequest{
		Operation: "extract_candidates",
		System:    CandidateExtractionSystemPrompt,
		User:      BuildCandidateExtractionUserPrompt(intent, location, results),
		MaxTokens: 800,
	}, &output); err != nil {
		return nil, err
	}

	candidates := cleanCandidates(output.Candidates)
	if len(candidates) > 0 {
		return candidates, nil
	}

	return fallbackCandidates(discovery.Results), nil
}

func (w *Workflow) researchCandidates(ctx context.Context, candidates []Candidate, intent Intent, location string) ([]RestaurantResult, []string) {
	if len(candidates) == 0 {
		return nil, []string{"no_candidates_found"}
	}

	results := make([]RestaurantResult, len(candidates))
	warnings := make([]string, 0)
	sem := make(chan struct{}, w.config.ResearchConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for index, candidate := range candidates {
		wg.Add(1)
		go func(index int, candidate Candidate) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			log.Printf("workflow candidate research started candidate=%q", candidate.Name)
			result, err := w.researchCandidate(ctx, candidate, intent, location)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				log.Printf("workflow candidate research failed candidate=%q duration=%s error=%v", candidate.Name, time.Since(start).Round(time.Millisecond), err)
				warnings = append(warnings, "research_failed:"+candidate.Name)
				return
			}
			log.Printf("workflow candidate research complete candidate=%q matched_items=%d confidence=%s duration=%s", candidate.Name, len(result.MenuItems), result.Confidence, time.Since(start).Round(time.Millisecond))
			results[index] = result
		}(index, candidate)
	}

	wg.Wait()
	return results, warnings
}

func (w *Workflow) researchCandidate(ctx context.Context, candidate Candidate, intent Intent, location string) (RestaurantResult, error) {
	menuSearch, err := w.search.SearchMenuSources(ctx, exa.MenuSearchRequest{
		Candidate:           candidate.Name,
		FoodQuery:           intent.FoodQuery,
		Location:            location,
		DietaryRestrictions: intent.DietaryRestrictions,
	}, 4)
	if err != nil {
		return RestaurantResult{}, err
	}

	sources := promptResults(menuSearch.Results, 3500)
	var output struct {
		RestaurantName        string     `json:"restaurantName"`
		HasSuitableItems      bool       `json:"hasSuitableItems"`
		DistanceText          string     `json:"distanceText"`
		MenuItems             []MenuItem `json:"menuItems"`
		DietaryAccommodations []string   `json:"dietaryAccommodations"`
		MenuURL               string     `json:"menuUrl"`
		SourceURLs            []string   `json:"sourceUrls"`
		Confidence            string     `json:"confidence"`
		Notes                 string     `json:"notes"`
	}

	if err := w.llm.ChatJSON(ctx, llm.ChatJSONRequest{
		Operation: "extract_menu_items",
		System:    MenuExtractionSystemPrompt,
		User:      BuildMenuExtractionUserPrompt(candidate, intent, location, sources),
		MaxTokens: 900,
	}, &output); err != nil {
		return RestaurantResult{}, err
	}

	sourceURLs := output.SourceURLs
	for _, source := range sources {
		sourceURLs = append(sourceURLs, source.URL)
	}

	name := firstNonEmpty(output.RestaurantName, candidate.Name)
	return RestaurantResult{
		Name:                  name,
		Source:                "new_find",
		DistanceText:          strings.TrimSpace(output.DistanceText),
		HasSuitableItems:      output.HasSuitableItems || len(output.MenuItems) > 0,
		MenuItems:             cleanMenuItems(output.MenuItems, intent.DietaryRestrictions),
		DietaryAccommodations: unique(output.DietaryAccommodations),
		MenuURL:               firstNonEmpty(output.MenuURL, firstSourceURL(sources), candidate.URL),
		SourceURLs:            unique(sourceURLs),
		Confidence:            normalizeConfidence(output.Confidence),
		Notes:                 strings.TrimSpace(output.Notes),
	}, nil
}

func normalizeRequest(raw FindFoodRequest) (FindFoodRequest, error) {
	raw.ConversationID = strings.TrimSpace(raw.ConversationID)
	raw.Message = strings.TrimSpace(firstNonEmpty(raw.Message, raw.Prompt))
	raw.Location = strings.TrimSpace(raw.Location)
	raw.DietaryRestrictions = unique(raw.DietaryRestrictions)
	if raw.Message == "" {
		return FindFoodRequest{}, apperrors.New(400, "bad_request", "Request body must include a non-empty message.")
	}
	return raw, nil
}

func (w *Workflow) loadConversation(request FindFoodRequest) (string, ConversationContext) {
	id := strings.TrimSpace(request.ConversationID)
	if id == "" {
		id = newConversationID()
	}

	conversation, ok := w.conversations.get(id)
	if !ok {
		conversation = ConversationContext{ConversationID: id}
	}
	conversation.ConversationID = id
	return id, conversation
}

func (w *Workflow) saveNeedsInputConversation(conversation ConversationContext, request FindFoodRequest, decision AgentDecision, missingFields []string, question string) {
	conversation.Messages = appendConversationMessage(conversation.Messages, "user", request.Message)
	conversation.Messages = appendConversationMessage(conversation.Messages, "assistant", question)
	conversation.Messages = trimConversationMessages(conversation.Messages, 12)
	conversation.KnownFields = mergeKnownFields(
		conversation.KnownFields,
		knownFieldsFromRequest(request),
		decision.KnownFields,
		knownFieldsFromToolRequest(decision.ToolRequest),
	)
	conversation.MissingFields = normalizeMissingFields(missingFields)
	conversation.LastFollowUpQuestion = question
	w.conversations.save(conversation)
}

func (w *Workflow) saveCompleteConversation(conversation ConversationContext, request FindFoodRequest, intent Intent) {
	conversation.Messages = appendConversationMessage(conversation.Messages, "user", request.Message)
	conversation.Messages = appendConversationMessage(conversation.Messages, "assistant", "Search completed.")
	conversation.Messages = trimConversationMessages(conversation.Messages, 12)
	conversation.KnownFields = mergeKnownFields(conversation.KnownFields, knownFieldsFromRequest(request), knownFieldsFromIntent(intent))
	conversation.MissingFields = nil
	conversation.LastFollowUpQuestion = ""
	w.conversations.save(conversation)
}

func resolveLocation(request FindFoodRequest, intent Intent) string {
	if request.Location != "" {
		return request.Location
	}
	if strings.TrimSpace(intent.Location) != "" {
		return strings.TrimSpace(intent.Location)
	}
	if request.ClientLocation != nil && strings.TrimSpace(request.ClientLocation.Label) != "" {
		return strings.TrimSpace(request.ClientLocation.Label)
	}
	if request.ClientLocation != nil && request.ClientLocation.Latitude != nil && request.ClientLocation.Longitude != nil {
		return strings.TrimSpace(floatString(*request.ClientLocation.Latitude) + "," + floatString(*request.ClientLocation.Longitude))
	}
	return ""
}

func intentFromAgentDecision(request FindFoodRequest, conversation ConversationContext, decision AgentDecision) Intent {
	return Intent{
		FoodQuery:           firstNonEmpty(decision.ToolRequest.FoodQuery, decision.KnownFields.FoodQuery, conversation.KnownFields.FoodQuery),
		LocationIntent:      normalizeLocationIntent(firstNonEmpty(decision.ToolRequest.LocationIntent, decision.KnownFields.LocationIntent, conversation.KnownFields.LocationIntent)),
		Location:            firstNonEmpty(request.Location, decision.ToolRequest.Location, decision.KnownFields.Location, conversation.KnownFields.Location),
		DietaryRestrictions: dietaryRestrictionsFromAgent(request, conversation, decision),
		Preferences:         unique(append(append(conversation.KnownFields.Preferences, decision.KnownFields.Preferences...), decision.ToolRequest.Preferences...)),
	}
}

func dietaryRestrictionsFromAgent(request FindFoodRequest, conversation ConversationContext, decision AgentDecision) []string {
	switch {
	case len(decision.ToolRequest.DietaryRestrictions) > 0:
		return unique(decision.ToolRequest.DietaryRestrictions)
	case len(decision.KnownFields.DietaryRestrictions) > 0:
		return unique(decision.KnownFields.DietaryRestrictions)
	case len(request.DietaryRestrictions) > 0:
		return unique(request.DietaryRestrictions)
	default:
		return unique(conversation.KnownFields.DietaryRestrictions)
	}
}

func knownFieldsFromRequest(request FindFoodRequest) AgentKnownFields {
	locationIntent := ""
	if request.ClientLocation != nil {
		locationIntent = "near_me"
	}
	if strings.TrimSpace(request.Location) != "" {
		locationIntent = "explicit"
	}

	return AgentKnownFields{
		Location:            resolveLocation(request, Intent{}),
		LocationIntent:      locationIntent,
		DietaryRestrictions: request.DietaryRestrictions,
	}
}

func knownFieldsFromToolRequest(request FindMenuItemsToolRequest) AgentKnownFields {
	return AgentKnownFields{
		FoodQuery:           request.FoodQuery,
		Location:            request.Location,
		LocationIntent:      request.LocationIntent,
		DietaryRestrictions: request.DietaryRestrictions,
		Preferences:         request.Preferences,
	}
}

func knownFieldsFromIntent(intent Intent) AgentKnownFields {
	return AgentKnownFields{
		FoodQuery:           intent.FoodQuery,
		Location:            intent.Location,
		LocationIntent:      intent.LocationIntent,
		DietaryRestrictions: intent.DietaryRestrictions,
		Preferences:         intent.Preferences,
	}
}

func mergeKnownFields(values ...AgentKnownFields) AgentKnownFields {
	var merged AgentKnownFields
	for _, value := range values {
		if strings.TrimSpace(value.FoodQuery) != "" {
			merged.FoodQuery = strings.TrimSpace(value.FoodQuery)
		}
		if strings.TrimSpace(value.Location) != "" {
			merged.Location = strings.TrimSpace(value.Location)
		}
		if strings.TrimSpace(value.LocationIntent) != "" && normalizeLocationIntent(value.LocationIntent) != "unspecified" {
			merged.LocationIntent = normalizeLocationIntent(value.LocationIntent)
		}
		if len(value.DietaryRestrictions) > 0 {
			merged.DietaryRestrictions = unique(value.DietaryRestrictions)
		}
		merged.Preferences = unique(append(merged.Preferences, value.Preferences...))
	}
	merged.LocationIntent = normalizeLocationIntent(merged.LocationIntent)
	return merged
}

func validateSearchIntent(intent Intent, location string) []string {
	missing := make([]string, 0, 3)
	if strings.TrimSpace(intent.FoodQuery) == "" {
		missing = append(missing, "foodQuery")
	}
	if strings.TrimSpace(location) == "" {
		missing = append(missing, "location")
	}
	if len(intent.DietaryRestrictions) == 0 {
		missing = append(missing, "dietaryRestrictions")
	}

	return normalizeMissingFields(missing)
}

func normalizeMissingFields(missing []string) []string {
	normalized := make([]string, 0, len(missing))
	for _, field := range unique(missing) {
		key := strings.ToLower(strings.TrimSpace(field))
		key = strings.NewReplacer(" ", "", "_", "", "-", "").Replace(key)
		switch key {
		case "foodquery", "food", "query", "craving", "dish", "item":
			normalized = append(normalized, "foodQuery")
		case "location", "place", "near", "nearme":
			normalized = append(normalized, "location")
		case "dietaryrestrictions", "restrictions", "allergies":
			normalized = append(normalized, "dietaryRestrictions")
		}
	}

	return unique(normalized)
}

func buildFollowUpQuestion(missing []string) string {
	has := map[string]bool{}
	for _, field := range missing {
		has[field] = true
	}

	switch {
	case has["foodQuery"] && has["location"] && has["dietaryRestrictions"]:
		return "What food are you looking for, where should I search, and what dietary restrictions should I apply?"
	case has["foodQuery"] && has["location"]:
		return "What food are you looking for, and where should I search?"
	case has["foodQuery"] && has["dietaryRestrictions"]:
		return "What food are you looking for, and what dietary restrictions should I apply?"
	case has["location"] && has["dietaryRestrictions"]:
		return "Where should I search, and what dietary restrictions should I apply?"
	case has["foodQuery"]:
		return "What food or dish are you looking for?"
	case has["location"]:
		return "Where should I search?"
	case has["dietaryRestrictions"]:
		return "What dietary restrictions should I apply?"
	default:
		return "What else should I know before searching?"
	}
}

func chooseFollowUpQuestion(decision AgentDecision, missing []string) string {
	question := strings.TrimSpace(decision.FollowUpQuestion)
	if question != "" && followUpQuestionCoversMissing(question, missing) {
		return question
	}
	return buildFollowUpQuestion(missing)
}

func followUpQuestionCoversMissing(question string, missing []string) bool {
	lower := strings.ToLower(question)
	for _, field := range normalizeMissingFields(missing) {
		switch field {
		case "foodQuery":
			if !strings.Contains(lower, "food") && !strings.Contains(lower, "dish") && !strings.Contains(lower, "craving") && !strings.Contains(lower, "cuisine") {
				return false
			}
		case "location":
			if !strings.Contains(lower, "where") && !strings.Contains(lower, "location") && !strings.Contains(lower, "near") && !strings.Contains(lower, "search") {
				return false
			}
		case "dietaryRestrictions":
			if !strings.Contains(lower, "diet") && !strings.Contains(lower, "restriction") && !strings.Contains(lower, "allerg") && !strings.Contains(lower, "avoid") {
				return false
			}
		}
	}
	return true
}

func missingWarnings(missing []string) []string {
	normalized := normalizeMissingFields(missing)
	warnings := make([]string, 0, len(normalized))
	for _, field := range normalized {
		warnings = append(warnings, "missing_"+field)
	}
	return warnings
}

func appendConversationMessage(messages []ConversationMessage, role string, content string) []ConversationMessage {
	content = strings.TrimSpace(content)
	if content == "" {
		return messages
	}
	return append(messages, ConversationMessage{
		Role:    role,
		Content: content,
	})
}

func trimConversationMessages(messages []ConversationMessage, limit int) []ConversationMessage {
	if limit <= 0 || len(messages) <= limit {
		return messages
	}
	return messages[len(messages)-limit:]
}

func promptResults(results []exa.Result, maxText int) []SearchResultForPrompt {
	out := make([]SearchResultForPrompt, 0, len(results))
	for _, result := range results {
		out = append(out, SearchResultForPrompt{
			Title:      result.Title,
			URL:        result.URL,
			Text:       truncate(result.Text, maxText),
			Highlights: result.Highlights,
		})
	}
	return out
}

func cleanCandidates(candidates []Candidate) []Candidate {
	out := make([]Candidate, 0, len(candidates))
	seen := map[string]bool{}
	for _, candidate := range candidates {
		candidate.Name = strings.TrimSpace(candidate.Name)
		if candidate.Name == "" {
			continue
		}
		key := strings.ToLower(candidate.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, candidate)
	}
	return out
}

func fallbackCandidates(results []exa.Result) []Candidate {
	candidates := make([]Candidate, 0, len(results))
	for _, result := range results {
		name := cleanCandidateName(result.Title)
		if name == "" {
			continue
		}
		candidates = append(candidates, Candidate{
			Name:   name,
			URL:    result.URL,
			Reason: "Fallback candidate from Exa result title",
		})
	}
	return cleanCandidates(candidates)
}

func limitCandidates(candidates []Candidate, limit int) []Candidate {
	if len(candidates) <= limit {
		return candidates
	}
	return candidates[:limit]
}

func filterSupportedRestaurants(restaurants []RestaurantResult) []RestaurantResult {
	out := make([]RestaurantResult, 0, len(restaurants))
	for _, restaurant := range restaurants {
		if restaurant.Name == "" || !restaurant.HasSuitableItems || len(restaurant.MenuItems) == 0 {
			continue
		}
		out = append(out, restaurant)
	}
	return out
}

func flattenFoodItems(restaurants []RestaurantResult) []FoodItemResult {
	items := make([]FoodItemResult, 0)
	for _, restaurant := range restaurants {
		for _, item := range restaurant.MenuItems {
			items = append(items, FoodItemResult{
				Name:                  item.Name,
				RestaurantName:        restaurant.Name,
				RestaurantSource:      restaurant.Source,
				DistanceText:          restaurant.DistanceText,
				WhyItFits:             item.WhyItFits,
				Caveats:               item.Caveats,
				DietaryAccommodations: restaurant.DietaryAccommodations,
				MenuURL:               restaurant.MenuURL,
				SourceURLs:            restaurant.SourceURLs,
				Confidence:            restaurant.Confidence,
				Notes:                 restaurant.Notes,
			})
		}
	}
	return items
}

func dedupeFoodItems(items []FoodItemResult) []FoodItemResult {
	out := make([]FoodItemResult, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.RestaurantName) + "::" + strings.TrimSpace(item.Name))
		if key == "::" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func dedupeRestaurants(restaurants []RestaurantResult) []RestaurantResult {
	out := make([]RestaurantResult, 0, len(restaurants))
	seen := map[string]bool{}
	for _, restaurant := range restaurants {
		key := strings.ToLower(strings.TrimSpace(restaurant.Name))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, restaurant)
	}
	return out
}

func cleanMenuItems(items []MenuItem, dietaryRestrictions []string) []MenuItem {
	out := make([]MenuItem, 0, len(items))
	for _, item := range items {
		item.Name = strings.TrimSpace(item.Name)
		item.WhyItFits = strings.TrimSpace(item.WhyItFits)
		item.Caveats = unique(item.Caveats)
		if item.Name != "" && !hasDisqualifyingCaveat(item, dietaryRestrictions) {
			out = append(out, item)
		}
	}
	return out
}

func hasDisqualifyingCaveat(item MenuItem, dietaryRestrictions []string) bool {
	if len(dietaryRestrictions) == 0 {
		return false
	}

	text := strings.ToLower(item.Name + " " + item.WhyItFits + " " + strings.Join(item.Caveats, " "))
	disqualifiers := []string{
		"not specifically marked",
		"not explicitly marked",
		"not confirmed",
		"verify ",
		"must verify",
		"verify with restaurant",
		"verify that this is",
		"ask restaurant about gluten-free",
		"ask the restaurant about gluten-free",
		"may contain gluten",
		"likely contains gluten",
		"would be gluten-free",
		"would be gluten free",
	}

	for _, disqualifier := range disqualifiers {
		if strings.Contains(text, disqualifier) {
			return true
		}
	}

	for _, restriction := range dietaryRestrictions {
		if strings.Contains(strings.ToLower(restriction), "gluten") &&
			(strings.Contains(text, "beer batter") ||
				strings.Contains(text, "beer-batter") ||
				strings.Contains(text, "beer battered") ||
				strings.Contains(text, "beer-battered") ||
				strings.Contains(text, "flour tortilla") ||
				strings.Contains(text, "wheat tortilla")) {
			return true
		}
	}

	return false
}

func cleanCandidateName(title string) string {
	replacer := strings.NewReplacer(" Menu", "", " menu", "", " Restaurant", "", " restaurant", "", " Official Site", "")
	name := replacer.Replace(strings.TrimSpace(title))
	for _, sep := range []string{" | ", " - "} {
		if before, _, ok := strings.Cut(name, sep); ok {
			name = before
		}
	}
	return strings.TrimSpace(name)
}

func firstSourceURL(sources []SearchResultForPrompt) string {
	for _, source := range sources {
		if strings.TrimSpace(source.URL) != "" {
			return strings.TrimSpace(source.URL)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeLocationIntent(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case "explicit", "near_me":
		return normalized
	default:
		return "unspecified"
	}
}

func normalizeConfidence(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "medium"
	}
}

func unique(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, trimmed)
	}
	return out
}

func truncate(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}
	return value[:maxLength]
}

func floatString(value float64) string {
	bytes, _ := json.Marshal(value)
	return string(bytes)
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func candidateNames(candidates []Candidate) []string {
	names := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		names = append(names, candidate.Name)
	}
	return names
}
