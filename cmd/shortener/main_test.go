package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/Bekw/go-musthave-shortener/internal/handler"
	"github.com/Bekw/go-musthave-shortener/internal/model"
	"go.uber.org/zap"
)

func newTestServer() (*chi.Mux, string) {
	base := "http://localhost:8080"
	logger := zap.NewNop()

	h := handler.NewHandler(model.NewMemoryStore(), base, logger)

	r := chi.NewRouter()
	r.Post("/", h.PostHandler)
	r.Post("/api/shorten", h.PostJSONHandler)
	r.Get("/{id}", h.GetHandler)
	return r, base
}

func createShort(t *testing.T, r *chi.Mux, original string) (string, string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(original))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("POST expected %d, got %d; body=%q", http.StatusCreated, res.StatusCode, string(b))
	}
	body, _ := io.ReadAll(res.Body)
	short := strings.TrimSpace(string(body))

	parts := strings.Split(short, "/")
	id := parts[len(parts)-1]
	return short, id
}

func TestPostHandler_Created(t *testing.T) {
	r, base := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("http://example.com"))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, res.StatusCode)
	}
	b, _ := io.ReadAll(res.Body)
	short := strings.TrimSpace(string(b))
	if !strings.HasPrefix(short, base+"/") {
		t.Fatalf("expected short url to start with %q, got %q", base+"/", short)
	}
}

func TestGetHandler_Redirect(t *testing.T) {
	r, _ := newTestServer()

	_, id := createShort(t, r, "http://example.com")

	req := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusTemporaryRedirect {
		t.Fatalf("expected %d, got %d", http.StatusTemporaryRedirect, res.StatusCode)
	}
	loc := res.Header.Get("Location")
	if loc != "http://example.com" {
		t.Fatalf("expected Location %q, got %q", "http://example.com", loc)
	}
}

func TestGetHandler_NotFound(t *testing.T) {
	r, _ := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/unknown123", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, res.StatusCode)
	}
}

func TestPostHandler_BadContentType(t *testing.T) {
	r, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("http://example.com"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, res.StatusCode)
	}
}

func TestPostHandler_EmptyBody(t *testing.T) {
	r, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, res.StatusCode)
	}
}
func TestPostJSONHandler_Created(t *testing.T) {
	r, _ := newTestServer()

	body := []byte(`{"url":"http://example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("expected %d, got %d", http.StatusCreated, res.StatusCode)
	}
	ct := res.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var got struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !strings.HasPrefix(got.Result, "http://localhost:8080/") {
		t.Fatalf("unexpected result: %s", got.Result)
	}
}

func TestPostJSONHandler_BadContentType(t *testing.T) {
	r, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader([]byte(`{"url":"http://example.com"}`)))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, res.StatusCode)
	}
}

func TestPostJSONHandler_InvalidJSON(t *testing.T) {
	r, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader([]byte(`{"url":""}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, res.StatusCode)
	}
}
