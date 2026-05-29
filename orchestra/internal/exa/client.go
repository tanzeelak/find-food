package exa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"find-restaurants/internal/apperrors"
)

type Client struct {
	apiKey string
	http   *http.Client
}

type RestaurantSearchRequest struct {
	FoodQuery           string
	Location            string
	DietaryRestrictions []string
}

type MenuSearchRequest struct {
	Candidate           string
	FoodQuery           string
	Location            string
	DietaryRestrictions []string
}

type SearchResponse struct {
	Query     string
	RequestID string
	Results   []Result
}

type Result struct {
	Title         string
	URL           string
	ID            string
	PublishedDate string
	Author        string
	Text          string
	Highlights    []string
}

func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		http: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (c *Client) SearchRestaurantCandidates(ctx context.Context, req RestaurantSearchRequest, numResults int) (SearchResponse, error) {
	restrictions := ""
	if len(req.DietaryRestrictions) > 0 {
		restrictions = " " + strings.Join(req.DietaryRestrictions, " ")
	}
	query := fmt.Sprintf("%s restaurants near %s%s menu", req.FoodQuery, req.Location, restrictions)
	return c.Search(ctx, query, numResults, false)
}

func (c *Client) SearchMenuSources(ctx context.Context, req MenuSearchRequest, numResults int) (SearchResponse, error) {
	restrictions := ""
	if len(req.DietaryRestrictions) > 0 {
		restrictions = " " + strings.Join(req.DietaryRestrictions, " ")
	}
	query := fmt.Sprintf("%s %s menu %s%s", req.Candidate, req.Location, req.FoodQuery, restrictions)
	return c.Search(ctx, query, numResults, true)
}

func (c *Client) Search(ctx context.Context, query string, numResults int, includeText bool) (SearchResponse, error) {
	if c.apiKey == "" {
		return SearchResponse{}, apperrors.New(500, "missing_exa_api_key", "Missing EXA_API_KEY for restaurant search.")
	}

	body := searchRequest{
		Query:      query,
		NumResults: numResults,
		Type:       "auto",
	}
	if includeText {
		body.Contents = map[string]any{
			"text": map[string]any{"maxCharacters": 3500},
		}
	}

	resp, err := c.postSearch(ctx, body)
	if err != nil {
		return SearchResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest && includeText {
		_ = resp.Body.Close()
		body.Contents = map[string]any{"text": true}
		resp, err = c.postSearch(ctx, body)
		if err != nil {
			return SearchResponse{}, err
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return SearchResponse{}, serviceError(resp, "exa_search_failed")
	}

	var decoded searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return SearchResponse{}, err
	}

	results := make([]Result, 0, len(decoded.Results))
	for _, result := range decoded.Results {
		url := result.URL
		if url == "" {
			url = result.ID
		}
		results = append(results, Result{
			Title:         result.Title,
			URL:           url,
			ID:            result.ID,
			PublishedDate: result.PublishedDate,
			Author:        result.Author,
			Text:          result.Text,
			Highlights:    result.Highlights,
		})
	}

	return SearchResponse{
		Query:     query,
		RequestID: decoded.RequestID,
		Results:   results,
	}, nil
}

func (c *Client) postSearch(ctx context.Context, body searchRequest) (*http.Response, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.exa.ai/search", bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, apperrors.New(502, "exa_search_failed", err.Error())
	}
	return resp, nil
}

type searchRequest struct {
	Query      string         `json:"query"`
	NumResults int            `json:"numResults"`
	Type       string         `json:"type"`
	Contents   map[string]any `json:"contents,omitempty"`
}

type searchResponse struct {
	RequestID string `json:"requestId"`
	Results   []struct {
		Title         string   `json:"title"`
		URL           string   `json:"url"`
		ID            string   `json:"id"`
		PublishedDate string   `json:"publishedDate"`
		Author        string   `json:"author"`
		Text          string   `json:"text"`
		Highlights    []string `json:"highlights"`
	} `json:"results"`
}

func serviceError(resp *http.Response, code string) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 700))
	status := 500
	if resp.StatusCode >= 500 {
		status = 502
	}
	return apperrors.New(status, code, fmt.Sprintf("Exa request failed with %d: %s", resp.StatusCode, strings.TrimSpace(string(body))))
}
