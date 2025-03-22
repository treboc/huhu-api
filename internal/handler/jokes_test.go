package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/treboc/huhu-api/internal/handler"
	"github.com/treboc/huhu-api/internal/model"
	"github.com/treboc/huhu-api/internal/repository"
)

// MockJokeRepository implements the JokeRepository interface for testing
type MockJokeRepository struct {
	jokes     []*model.Joke
	nextID    int64
	lastError error
}

func NewMockJokeRepository() *MockJokeRepository {
	return &MockJokeRepository{
		jokes:  make([]*model.Joke, 0),
		nextID: 1,
	}
}

func (r *MockJokeRepository) GetJoke(ctx context.Context, id int64) (*model.Joke, error) {
	if r.lastError != nil {
		return nil, r.lastError
	}

	for _, joke := range r.jokes {
		if joke.ID == id {
			return joke, nil
		}
	}

	return nil, repository.ErrJokeNotFound
}

func (r *MockJokeRepository) GetRandomJoke(ctx context.Context) (*model.Joke, error) {
	if r.lastError != nil {
		return nil, r.lastError
	}

	if len(r.jokes) == 0 {
		return nil, repository.ErrNoJokes
	}

	return r.jokes[0], nil // For testing, just return the first joke
}

func (r *MockJokeRepository) ListJokes(ctx context.Context, limit, offset int) ([]*model.Joke, error) {
	if r.lastError != nil {
		return nil, r.lastError
	}

	if offset >= len(r.jokes) {
		return []*model.Joke{}, nil
	}

	end := offset + limit
	if end > len(r.jokes) {
		end = len(r.jokes)
	}

	return r.jokes[offset:end], nil
}

func (r *MockJokeRepository) CreateJoke(ctx context.Context, joke *model.Joke) (int64, error) {
	if r.lastError != nil {
		return 0, r.lastError
	}

	joke.ID = r.nextID
	joke.CreatedAt = time.Now()
	joke.UpdatedAt = time.Now()
	r.jokes = append(r.jokes, joke)
	r.nextID++

	return joke.ID, nil
}

func (r *MockJokeRepository) UpdateJoke(ctx context.Context, joke *model.Joke) error {
	if r.lastError != nil {
		return r.lastError
	}

	for i, j := range r.jokes {
		if j.ID == joke.ID {
			joke.CreatedAt = j.CreatedAt
			joke.UpdatedAt = time.Now()
			r.jokes[i] = joke
			return nil
		}
	}

	return repository.ErrJokeNotFound
}

func (r *MockJokeRepository) DeleteJoke(ctx context.Context, id int64) error {
	if r.lastError != nil {
		return r.lastError
	}

	for i, joke := range r.jokes {
		if joke.ID == id {
			r.jokes = append(r.jokes[:i], r.jokes[i+1:]...)
			return nil
		}
	}

	return repository.ErrJokeNotFound
}

func (r *MockJokeRepository) CountJokes(ctx context.Context) (int, error) {
	if r.lastError != nil {
		return 0, r.lastError
	}

	return len(r.jokes), nil
}

func (r *MockJokeRepository) Close() error {
	return nil
}

// SetError allows tests to make the mock repository return a specific error
func (r *MockJokeRepository) SetError(err error) {
	r.lastError = err
}

// ResetError clears any error set on the mock repository
func (r *MockJokeRepository) ResetError() {
	r.lastError = nil
}

// Test setup functions
func setupJokeHandler() (*handler.JokeHandler, *MockJokeRepository) {
	mockRepo := NewMockJokeRepository()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Add some test jokes
	now := time.Now()
	mockRepo.jokes = append(mockRepo.jokes,
		&model.Joke{ID: 1, Text: "Why don't scientists trust atoms? Because they make up everything!", CreatedAt: now, UpdatedAt: now},
		&model.Joke{ID: 2, Text: "Did you hear about the mathematician who's afraid of negative numbers? He'll stop at nothing to avoid them.", CreatedAt: now, UpdatedAt: now},
		&model.Joke{ID: 3, Text: "Why did the scarecrow win an award? Because he was outstanding in his field!", CreatedAt: now, UpdatedAt: now},
	)
	mockRepo.nextID = 4

	return handler.NewJokeHandler(mockRepo, logger), mockRepo
}

// Test cases
func TestListJokes(t *testing.T) {
	jokeHandler, _ := setupJokeHandler()

	tests := []struct {
		name      string
		url       string
		wantCode  int
		wantJokes int
	}{
		{
			name:      "default pagination",
			url:       "/api/joke",
			wantCode:  http.StatusOK,
			wantJokes: 3,
		},
		{
			name:      "with limit",
			url:       "/api/joke?limit=2",
			wantCode:  http.StatusOK,
			wantJokes: 2,
		},
		{
			name:      "with offset",
			url:       "/api/joke?offset=2",
			wantCode:  http.StatusOK,
			wantJokes: 1,
		},
		{
			name:      "with limit and offset",
			url:       "/api/joke?limit=1&offset=1",
			wantCode:  http.StatusOK,
			wantJokes: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", tt.url, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			h := http.HandlerFunc(jokeHandler.ListJokes)

			h.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.wantCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.wantCode)
			}

			var response handler.JokeListResponse
			err = json.Unmarshal(rr.Body.Bytes(), &response)
			if err != nil {
				t.Fatal(err)
			}

			if len(response.Jokes) != tt.wantJokes {
				t.Errorf("expected %d jokes, got %d", tt.wantJokes, len(response.Jokes))
			}

			if response.Total != 3 {
				t.Errorf("expected total to be 3, got %d", response.Total)
			}
		})
	}
}

func TestGetJoke(t *testing.T) {
	jokeHandler, mockRepo := setupJokeHandler()

	tests := []struct {
		name     string
		jokeID   string
		wantCode int
		setError error
	}{
		{
			name:     "existing joke",
			jokeID:   "1",
			wantCode: http.StatusOK,
		},
		{
			name:     "non-existent joke",
			jokeID:   "999",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "invalid joke ID",
			jokeID:   "abc",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "server error",
			jokeID:   "1",
			wantCode: http.StatusInternalServerError,
			setError: errors.New("database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset or set error as needed
			if tt.setError != nil {
				mockRepo.SetError(tt.setError)
			} else {
				mockRepo.ResetError()
			}

			// Create a new request with a URL parameter
			req, err := http.NewRequest("GET", "/api/joke/"+tt.jokeID, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Set up the router context with URL parameters
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.jokeID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(jokeHandler.GetJoke)

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.wantCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.wantCode)
			}

			if tt.wantCode == http.StatusOK {
				var joke model.Joke
				err = json.Unmarshal(rr.Body.Bytes(), &joke)
				if err != nil {
					t.Fatal(err)
				}

				id, _ := strconv.ParseInt(tt.jokeID, 10, 64)
				if joke.ID != id {
					t.Errorf("expected joke ID %d, got %d", id, joke.ID)
				}
			}
		})
	}
}

func TestGetRandomJoke(t *testing.T) {
	jokeHandler, mockRepo := setupJokeHandler()

	tests := []struct {
		name      string
		wantCode  int
		setError  error
		emptyRepo bool
	}{
		{
			name:     "success",
			wantCode: http.StatusOK,
		},
		{
			name:      "no jokes available",
			wantCode:  http.StatusNotFound,
			emptyRepo: true,
		},
		{
			name:     "server error",
			wantCode: http.StatusInternalServerError,
			setError: errors.New("database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset or set error as needed
			if tt.setError != nil {
				mockRepo.SetError(tt.setError)
			} else {
				mockRepo.ResetError()
			}

			// Empty the repository if needed
			if tt.emptyRepo {
				mockRepo.jokes = []*model.Joke{}
			} else if len(mockRepo.jokes) == 0 {
				// Restore test jokes if needed
				_, mockRepo = setupJokeHandler()
			}

			req, err := http.NewRequest("GET", "/api/joke/random", nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(jokeHandler.GetRandomJoke)

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.wantCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.wantCode)
			}

			if tt.wantCode == http.StatusOK {
				var joke model.Joke
				err = json.Unmarshal(rr.Body.Bytes(), &joke)
				if err != nil {
					t.Fatal(err)
				}

				if joke.ID == 0 || joke.Text == "" {
					t.Error("expected non-empty joke")
				}
			}
		})
	}
}

func TestCreateJoke(t *testing.T) {
	jokeHandler, mockRepo := setupJokeHandler()

	tests := []struct {
		name     string
		payload  string
		wantCode int
		setError error
	}{
		{
			name:     "valid joke",
			payload:  `{"text":"Why did the chicken cross the road? To get to the other side!"}`,
			wantCode: http.StatusCreated,
		},
		{
			name:     "empty joke",
			payload:  `{"text":""}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "invalid JSON",
			payload:  `{"text":}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "server error",
			payload:  `{"text":"Why did the chicken cross the road? To get to the other side!"}`,
			wantCode: http.StatusInternalServerError,
			setError: errors.New("database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset or set error as needed
			if tt.setError != nil {
				mockRepo.SetError(tt.setError)
			} else {
				mockRepo.ResetError()
			}

			req, err := http.NewRequest("POST", "/api/admin/joke", bytes.NewBufferString(tt.payload))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(jokeHandler.CreateJoke)

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.wantCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.wantCode)
			}

			if tt.wantCode == http.StatusCreated {
				var joke model.Joke
				err = json.Unmarshal(rr.Body.Bytes(), &joke)
				if err != nil {
					t.Fatal(err)
				}

				if joke.ID == 0 || joke.Text == "" {
					t.Error("expected non-empty joke")
				}

				if joke.ID != 4 { // Next ID should be 4
					t.Errorf("expected joke ID 4, got %d", joke.ID)
				}
			}
		})
	}
}

func TestUpdateJoke(t *testing.T) {
	jokeHandler, mockRepo := setupJokeHandler()

	tests := []struct {
		name     string
		jokeID   string
		payload  string
		wantCode int
		setError error
	}{
		{
			name:     "valid update",
			jokeID:   "1",
			payload:  `{"text":"Updated joke text"}`,
			wantCode: http.StatusOK,
		},
		{
			name:     "non-existent joke",
			jokeID:   "999",
			payload:  `{"text":"This won't be updated"}`,
			wantCode: http.StatusNotFound,
		},
		{
			name:     "invalid joke ID",
			jokeID:   "abc",
			payload:  `{"text":"This won't be updated"}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "empty joke text",
			jokeID:   "1",
			payload:  `{"text":""}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "invalid JSON",
			jokeID:   "1",
			payload:  `{"text":}`,
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "server error",
			jokeID:   "1",
			payload:  `{"text":"This should cause an error"}`,
			wantCode: http.StatusInternalServerError,
			setError: errors.New("database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset or set error as needed
			if tt.setError != nil {
				mockRepo.SetError(tt.setError)
			} else {
				mockRepo.ResetError()
			}

			req, err := http.NewRequest("PUT", "/api/admin/joke/"+tt.jokeID, bytes.NewBufferString(tt.payload))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			// Set up the router context with URL parameters
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.jokeID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(jokeHandler.UpdateJoke)

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.wantCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.wantCode)
			}

			if tt.wantCode == http.StatusOK {
				var joke model.Joke
				err = json.Unmarshal(rr.Body.Bytes(), &joke)
				if err != nil {
					t.Fatal(err)
				}

				id, _ := strconv.ParseInt(tt.jokeID, 10, 64)
				if joke.ID != id {
					t.Errorf("expected joke ID %d, got %d", id, joke.ID)
				}

				var requestJoke struct {
					Text string `json:"text"`
				}
				json.Unmarshal([]byte(tt.payload), &requestJoke)

				if joke.Text != requestJoke.Text {
					t.Errorf("expected joke text '%s', got '%s'", requestJoke.Text, joke.Text)
				}
			}
		})
	}
}

func TestDeleteJoke(t *testing.T) {
	jokeHandler, mockRepo := setupJokeHandler()

	tests := []struct {
		name     string
		jokeID   string
		wantCode int
		setError error
	}{
		{
			name:     "valid deletion",
			jokeID:   "1",
			wantCode: http.StatusNoContent,
		},
		{
			name:     "non-existent joke",
			jokeID:   "999",
			wantCode: http.StatusNotFound,
		},
		{
			name:     "invalid joke ID",
			jokeID:   "abc",
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "server error",
			jokeID:   "2",
			wantCode: http.StatusInternalServerError,
			setError: errors.New("database error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset or set error as needed
			if tt.setError != nil {
				mockRepo.SetError(tt.setError)
			} else {
				mockRepo.ResetError()
			}

			req, err := http.NewRequest("DELETE", "/api/admin/joke/"+tt.jokeID, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Set up the router context with URL parameters
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.jokeID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(jokeHandler.DeleteJoke)

			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.wantCode {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.wantCode)
			}

			if tt.wantCode == http.StatusNoContent && tt.setError == nil {
				// Check that the joke was actually deleted
				id, _ := strconv.ParseInt(tt.jokeID, 10, 64)
				joke, err := mockRepo.GetJoke(context.Background(), id)
				if err == nil || joke != nil {
					t.Errorf("joke with ID %d should have been deleted", id)
				}
			}
		})
	}
}
