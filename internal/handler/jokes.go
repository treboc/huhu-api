package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/treboc/huhu-api/internal/model"
	"github.com/treboc/huhu-api/internal/repository"
)

type JokeHandler struct {
	repo   repository.JokeRepository
	logger *slog.Logger
}

func NewJokeHandler(repo repository.JokeRepository, logger *slog.Logger) *JokeHandler {
	return &JokeHandler{
		repo:   repo,
		logger: logger,
	}
}

type JokeListResponse struct {
	Jokes  []*model.Joke `json:"jokes"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (h *JokeHandler) ListJokes(w http.ResponseWriter, r *http.Request) {
	limit := 10 // Default limit
	offset := 0 // Default offset

	limitParam := r.URL.Query().Get("limit")
	if limitParam != "" {
		parsedLimit, err := strconv.Atoi(limitParam)
		if err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	offsetParam := r.URL.Query().Get("offset")
	if offsetParam != "" {
		parsedOffset, err := strconv.Atoi(offsetParam)
		if err == nil && parsedOffset >= 0 {
			offset = parsedOffset
		}
	}

	jokes, err := h.repo.ListJokes(r.Context(), limit, offset)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve jokes")
		return
	}

	total, err := h.repo.CountJokes(r.Context())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to count jokes")
		return
	}

	response := JokeListResponse{
		Jokes:  jokes,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}

	respondWithJSON(w, http.StatusOK, response)
}

func (h *JokeHandler) GetJoke(w http.ResponseWriter, r *http.Request) {
	// Parse joke ID from URL
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid joke ID")
		return
	}

	joke, err := h.repo.GetJoke(r.Context(), id)
	if err != nil {
		// Check if it's a "not found" error
		if errors.Is(err, repository.ErrJokeNotFound) {
			respondWithError(w, http.StatusNotFound, "Joke not found")
			return
		}

		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve joke")
		return
	}

	respondWithJSON(w, http.StatusOK, joke)
}

func (h *JokeHandler) GetRandomJoke(w http.ResponseWriter, r *http.Request) {
	joke, err := h.repo.GetRandomJoke(r.Context())
	if err != nil {
		if errors.Is(err, repository.ErrNoJokes) {
			respondWithError(w, http.StatusNotFound, "No jokes available")
			return
		}

		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve random joke")
		return
	}

	respondWithJSON(w, http.StatusOK, joke)
}

type CreateJokeRequest struct {
	Text string `json:"text"`
}

// CreateJoke handles POST /api/admin/jokes
func (h *JokeHandler) CreateJoke(w http.ResponseWriter, r *http.Request) {
	var req CreateJokeRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Text == "" {
		respondWithError(w, http.StatusBadRequest, "Joke text is required")
		return
	}

	joke := &model.Joke{
		Text: req.Text,
	}

	id, err := h.repo.CreateJoke(r.Context(), joke)
	if err != nil {
		h.logger.Error("Failed to create joke", slog.String("error", err.Error()))
		respondWithError(w, http.StatusInternalServerError, "Failed to create joke")
		return
	}

	// Get the created joke
	createdJoke, err := h.repo.GetJoke(r.Context(), id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Joke created but failed to retrieve")
		return
	}

	w.Header().Set("Location", "/api/jokes/"+strconv.FormatInt(id, 10))
	respondWithJSON(w, http.StatusCreated, createdJoke)
}

func (h *JokeHandler) UpdateJoke(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid joke ID")
		return
	}

	var req CreateJokeRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Text == "" {
		respondWithError(w, http.StatusBadRequest, "Joke text is required")
		return
	}

	_, err = h.repo.GetJoke(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrJokeNotFound) {
			respondWithError(w, http.StatusNotFound, "Joke not found")
			return
		}

		respondWithError(w, http.StatusInternalServerError, "Failed to retrieve joke")
		return
	}

	joke := &model.Joke{
		ID:   id,
		Text: req.Text,
	}

	if err := h.repo.UpdateJoke(r.Context(), joke); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update joke")
		return
	}

	updatedJoke, err := h.repo.GetJoke(r.Context(), id)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Joke updated but failed to retrieve")
		return
	}

	respondWithJSON(w, http.StatusOK, updatedJoke)
}

func (h *JokeHandler) DeleteJoke(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idParam, 10, 64)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid joke ID")
		return
	}

	if err := h.repo.DeleteJoke(r.Context(), id); err != nil {
		if errors.Is(err, repository.ErrJokeNotFound) {
			respondWithError(w, http.StatusNotFound, "Joke not found")
			return
		}

		respondWithError(w, http.StatusInternalServerError, "Failed to delete joke")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, ErrorResponse{
		Error: message,
	})
}
