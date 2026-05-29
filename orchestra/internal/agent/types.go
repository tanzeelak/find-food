package agent

type FindFoodRequest struct {
	Message             string          `json:"message"`
	Prompt              string          `json:"prompt,omitempty"`
	Location            string          `json:"location,omitempty"`
	DietaryRestrictions []string        `json:"dietaryRestrictions,omitempty"`
	ClientLocation      *ClientLocation `json:"clientLocation,omitempty"`
}

type ClientLocation struct {
	Label     string   `json:"label,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

type FindFoodResponse struct {
	Status           string            `json:"status"`
	Items            []FoodItemResult  `json:"items"`
	FollowUpQuestion *string           `json:"followUpQuestion"`
	Warnings         []string          `json:"warnings,omitempty"`
	Metadata         *ResponseMetadata `json:"metadata,omitempty"`
}

type ResponseMetadata struct {
	Location            string   `json:"location"`
	FoodQuery           string   `json:"foodQuery"`
	DietaryRestrictions []string `json:"dietaryRestrictions"`
	DiscoveryQuery      string   `json:"discoveryQuery"`
	CandidateCount      int      `json:"candidateCount"`
}

type Intent struct {
	FoodQuery           string   `json:"foodQuery"`
	LocationIntent      string   `json:"locationIntent"`
	Location            string   `json:"location"`
	DietaryRestrictions []string `json:"dietaryRestrictions"`
	Preferences         []string `json:"preferences"`
	MissingFields       []string `json:"missingFields"`
	FollowUpQuestion    *string  `json:"followUpQuestion"`
}

type Candidate struct {
	Name         string `json:"name"`
	Neighborhood string `json:"neighborhood,omitempty"`
	URL          string `json:"url,omitempty"`
	Reason       string `json:"reason,omitempty"`
}

type MenuItem struct {
	Name      string   `json:"name"`
	WhyItFits string   `json:"whyItFits,omitempty"`
	Caveats   []string `json:"caveats,omitempty"`
}

type FoodItemResult struct {
	Name                  string   `json:"name"`
	RestaurantName        string   `json:"restaurantName"`
	RestaurantSource      string   `json:"restaurantSource"`
	DistanceText          string   `json:"distanceText,omitempty"`
	WhyItFits             string   `json:"whyItFits,omitempty"`
	Caveats               []string `json:"caveats,omitempty"`
	DietaryAccommodations []string `json:"dietaryAccommodations"`
	MenuURL               string   `json:"menuUrl,omitempty"`
	SourceURLs            []string `json:"sourceUrls"`
	Confidence            string   `json:"confidence"`
	Notes                 string   `json:"notes,omitempty"`
}

type RestaurantResult struct {
	Name                  string     `json:"name"`
	Source                string     `json:"source"`
	DistanceText          string     `json:"distanceText,omitempty"`
	HasSuitableItems      bool       `json:"hasSuitableItems"`
	MenuItems             []MenuItem `json:"menuItems"`
	DietaryAccommodations []string   `json:"dietaryAccommodations"`
	MenuURL               string     `json:"menuUrl,omitempty"`
	SourceURLs            []string   `json:"sourceUrls"`
	Confidence            string     `json:"confidence"`
	Notes                 string     `json:"notes,omitempty"`
}
