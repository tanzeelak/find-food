package api

import (
	"encoding/json"
	"errors"
	"net/http"

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
	var request agent.FindFoodRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, apperrors.New(http.StatusBadRequest, "invalid_json", "Request body must be valid JSON matching the find-food request shape."))
		return
	}

	response, err := s.workflow.Run(r.Context(), request)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")

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
	_ = json.NewEncoder(w).Encode(payload)
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
