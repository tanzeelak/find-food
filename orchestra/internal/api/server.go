package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"find-restaurants/internal/agent"
	"find-restaurants/internal/apperrors"
)

type Server struct {
	workflow *agent.Workflow
}

func NewServer(workflow *agent.Workflow) *Server {
	return &Server{workflow: workflow}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("POST /api/find-food", s.handleFindFood)

	return cors(mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"service":  "find-food",
		"codebuff": false,
		"endpoints": map[string]string{
			"health":   "GET /health",
			"findFood": "POST /api/find-food",
		},
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"service":  "find-food",
		"codebuff": false,
	})
}

func (s *Server) handleFindFood(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	log.Printf("find-food request started remote=%s", r.RemoteAddr)

	var request agent.FindFoodRequest
	if err := decodeFindFoodRequest(w, r, &request); err != nil {
		log.Printf("find-food request rejected duration=%s error=invalid_json", time.Since(start).Round(time.Millisecond))
		writeError(w, apperrors.New(http.StatusBadRequest, "invalid_request", "Request body must be a non-empty plain text prompt, JSON string, or JSON object matching the find-food request shape."))
		return
	}
	applyConversationID(r, &request)

	response, err := s.workflow.Run(r.Context(), request)
	if err != nil {
		log.Printf("find-food request failed duration=%s error=%v", time.Since(start).Round(time.Millisecond), err)
		writeError(w, err)
		return
	}

	log.Printf("find-food request complete duration=%s status=%s items=%d", time.Since(start).Round(time.Millisecond), response.Status, len(response.Items))
	writeJSON(w, http.StatusOK, response)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Conversation-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func decodeFindFoodRequest(w http.ResponseWriter, r *http.Request, request *agent.FindFoodRequest) error {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		return err
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return errors.New("empty body")
	}

	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "text/plain") || (trimmed[0] != '{' && trimmed[0] != '"') {
		request.Message = trimmed
		return nil
	}

	if trimmed[0] == '"' {
		var message string
		if err := json.Unmarshal(body, &message); err != nil {
			return err
		}
		request.Message = strings.TrimSpace(message)
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	return decoder.Decode(request)
}

func applyConversationID(r *http.Request, request *agent.FindFoodRequest) {
	if strings.TrimSpace(request.ConversationID) != "" {
		request.ConversationID = strings.TrimSpace(request.ConversationID)
		return
	}
	if value := strings.TrimSpace(r.Header.Get("X-Conversation-ID")); value != "" {
		request.ConversationID = value
		return
	}
	if value := strings.TrimSpace(r.URL.Query().Get("conversationId")); value != "" {
		request.ConversationID = value
	}
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *apperrors.Error
	if errors.As(err, &appErr) {
		writeJSON(w, appErr.Status, map[string]any{
			"status": "error",
			"error": map[string]string{
				"code":    appErr.Code,
				"message": appErr.Detail,
			},
		})
		return
	}

	writeJSON(w, http.StatusInternalServerError, map[string]any{
		"status": "error",
		"error": map[string]string{
			"code":    "internal_error",
			"message": err.Error(),
		},
	})
}
