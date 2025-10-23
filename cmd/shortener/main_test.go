package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostHandler(t *testing.T) {
	body := strings.NewReader("http://example.com")

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	postHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		t.Errorf("expected %d, got %d", http.StatusCreated, res.StatusCode)
	}

	respBody, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(respBody), "http://localhost:8080/") {
		t.Errorf("expected short url, got %s", string(respBody))
	}
}
func TestGetHandler(t *testing.T) {
	id := "abc123"
	urlStore[id] = "http://example.com"

	req := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	w := httptest.NewRecorder()
	getHandler(w, req)

	res := w.Result()
	defer res.Body.Close()

	if res.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("expected %d, got %d", http.StatusTemporaryRedirect, res.StatusCode)
	}

	loc := res.Header.Get("Location")
	if loc != "http://example.com" {
		t.Errorf("expected redirect to %s, got %s", "http://example.com", loc)
	}
}
