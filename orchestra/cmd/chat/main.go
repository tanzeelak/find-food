package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultAPIURL = "http://127.0.0.1:3000/api/find-food"

type findFoodRequest struct {
	ConversationID string `json:"conversationId,omitempty"`
	Message        string `json:"message"`
}

type findFoodResponse struct {
	ConversationID   string            `json:"conversationId"`
	Status           string            `json:"status"`
	Items            []foodItem        `json:"items"`
	FollowUpQuestion *string           `json:"followUpQuestion"`
	Warnings         []string          `json:"warnings,omitempty"`
	Metadata         *responseMetadata `json:"metadata,omitempty"`
	Error            *apiError         `json:"error,omitempty"`
}

type foodItem struct {
	Name           string   `json:"name"`
	RestaurantName string   `json:"restaurantName"`
	WhyItFits      string   `json:"whyItFits,omitempty"`
	Caveats        []string `json:"caveats,omitempty"`
	Confidence     string   `json:"confidence"`
	MenuURL        string   `json:"menuUrl,omitempty"`
}

type responseMetadata struct {
	Location            string   `json:"location"`
	FoodQuery           string   `json:"foodQuery"`
	DietaryRestrictions []string `json:"dietaryRestrictions"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func main() {
	apiURL := strings.TrimSpace(os.Getenv("FIND_FOOD_API_URL"))
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	client := &http.Client{Timeout: 2 * time.Minute}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	conversationID := ""
	fmt.Printf("Find Food chat. API: %s\n", apiURL)
	fmt.Println("Type quit to exit, or reset to start a new conversation.")

	for {
		fmt.Print("\nyou> ")
		if !scanner.Scan() {
			break
		}

		message := strings.TrimSpace(scanner.Text())
		switch strings.ToLower(message) {
		case "":
			continue
		case "quit", "exit":
			return
		case "reset":
			conversationID = ""
			fmt.Println("agent> Started a new conversation.")
			continue
		}

		response, err := sendMessage(client, apiURL, conversationID, message)
		if err != nil {
			fmt.Printf("agent> Request failed: %v\n", err)
			fmt.Println("agent> Make sure the API server is running with: go run ./cmd/api")
			continue
		}

		if response.ConversationID != "" {
			conversationID = response.ConversationID
		}

		printResponse(response)
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("agent> Input error: %v\n", err)
	}
}

func sendMessage(client *http.Client, apiURL string, conversationID string, message string) (findFoodResponse, error) {
	payload, err := json.Marshal(findFoodRequest{
		ConversationID: conversationID,
		Message:        message,
	})
	if err != nil {
		return findFoodResponse{}, err
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return findFoodResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return findFoodResponse{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return findFoodResponse{}, err
	}

	var output findFoodResponse
	if err := json.Unmarshal(body, &output); err != nil {
		return findFoodResponse{}, fmt.Errorf("decode response: %w: %s", err, strings.TrimSpace(string(body)))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if output.Error != nil {
			return findFoodResponse{}, fmt.Errorf("%s: %s", output.Error.Code, output.Error.Message)
		}
		return findFoodResponse{}, fmt.Errorf("unexpected HTTP status %s", resp.Status)
	}
	if output.Error != nil {
		return findFoodResponse{}, fmt.Errorf("%s: %s", output.Error.Code, output.Error.Message)
	}

	return output, nil
}

func printResponse(response findFoodResponse) {
	switch response.Status {
	case "needs_input":
		if response.FollowUpQuestion != nil && strings.TrimSpace(*response.FollowUpQuestion) != "" {
			fmt.Printf("agent> %s\n", strings.TrimSpace(*response.FollowUpQuestion))
			return
		}
		fmt.Println("agent> What else should I know?")
	case "complete":
		printCompleteResponse(response)
	default:
		fmt.Printf("agent> Status: %s\n", response.Status)
	}
}

func printCompleteResponse(response findFoodResponse) {
	if len(response.Items) == 0 {
		fmt.Println("agent> I did not find matching menu items.")
		return
	}

	fmt.Printf("agent> Found %d matching menu items", len(response.Items))
	if response.Metadata != nil {
		fmt.Printf(" for %s near %s", response.Metadata.FoodQuery, response.Metadata.Location)
	}
	fmt.Println(":")

	for index, item := range response.Items {
		fmt.Printf("\n%d. %s - %s", index+1, item.Name, item.RestaurantName)
		if item.Confidence != "" {
			fmt.Printf(" (%s confidence)", item.Confidence)
		}
		fmt.Println()
		if item.WhyItFits != "" {
			fmt.Printf("   %s\n", item.WhyItFits)
		}
		if len(item.Caveats) > 0 {
			fmt.Printf("   Caveats: %s\n", strings.Join(item.Caveats, "; "))
		}
		if item.MenuURL != "" {
			fmt.Printf("   Menu: %s\n", item.MenuURL)
		}
	}
}
