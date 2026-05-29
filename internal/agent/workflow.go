package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

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
	llm    LLMClient
	search SearchClient
	config config.WorkflowConfig
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
		llm:    llmClient,
		search: searchClient,
		config: cfg,
	}
}

func (w *Workflow) Run(ctx context.Context, raw FindFoodRequest) (FindFoodResponse, error) {
	request, err := normalizeRequest(raw)
	if err != nil {
		return FindFoodResponse{}, err
	}

	intent, err := w.parseIntent(ctx, request)
	if err != nil {
		return FindFoodResponse{}, err
	}

	location := resolveLocation(request, intent)
	if location == "" {
		question := "What location should I search near?"
		if intent.FollowUpQuestion != nil && strings.TrimSpace(*intent.FollowUpQuestion) != "" {
			question = strings.TrimSpace(*intent.FollowUpQuestion)
		}

		return FindFoodResponse{
			Status:           "needs_input",
			Items:            []FoodItemResult{},
			FollowUpQuestion: &question,
			Warnings:         []string{"location_required"},
		}, nil
	}

	discovery, err := w.search.SearchRestaurantCandidates(ctx, exa.RestaurantSearchRequest{
		FoodQuery:           intent.FoodQuery,
		Location:            location,
		DietaryRestrictions: intent.DietaryRestrictions,
	}, max(w.config.MaxCandidates, 8))
	if err != nil {
		return FindFoodResponse{}, err
	}

	candidates, err := w.extractCandidates(ctx, intent, location, discovery)
	if err != nil {
		return FindFoodResponse{}, err
	}
	candidates = limitCandidates(candidates, w.config.MaxCandidates)

	restaurants, warnings := w.researchCandidates(ctx, candidates, intent, location)
	restaurants = filterSupportedRestaurants(dedupeRestaurants(restaurants))
	items := dedupeFoodItems(flattenFoodItems(restaurants))
	if len(items) > w.config.MaxResults {
		items = items[:w.config.MaxResults]
	}
	if len(items) == 0 {
		warnings = append(warnings, "no_matching_menu_items_found")
	}

	return FindFoodResponse{
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

func (w *Workflow) parseIntent(ctx context.Context, request FindFoodRequest) (Intent, error) {
	var intent Intent
	if err := w.llm.ChatJSON(ctx, llm.ChatJSONRequest{
		Operation: "parse_intent",
		System:    IntentSystemPrompt,
		User:      BuildIntentUserPrompt(request),
		MaxTokens: 600,
	}, &intent); err != nil {
		return Intent{}, err
	}

	intent.FoodQuery = firstNonEmpty(intent.FoodQuery, request.Message)
	intent.Location = strings.TrimSpace(intent.Location)
	intent.LocationIntent = firstNonEmpty(intent.LocationIntent, "unspecified")
	intent.DietaryRestrictions = unique(append(request.DietaryRestrictions, intent.DietaryRestrictions...))
	intent.Preferences = unique(intent.Preferences)
	intent.MissingFields = unique(intent.MissingFields)

	return intent, nil
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

			result, err := w.researchCandidate(ctx, candidate, intent, location)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				warnings = append(warnings, "research_failed:"+candidate.Name)
				return
			}
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
	raw.Message = strings.TrimSpace(firstNonEmpty(raw.Message, raw.Prompt))
	raw.Location = strings.TrimSpace(raw.Location)
	raw.DietaryRestrictions = unique(raw.DietaryRestrictions)
	if raw.Message == "" {
		return FindFoodRequest{}, apperrors.New(400, "bad_request", "Request body must include a non-empty message.")
	}
	return raw, nil
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
