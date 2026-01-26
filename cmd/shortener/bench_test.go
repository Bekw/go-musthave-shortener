package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func warmUpPlain(r http.Handler, original string) *http.Cookie {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(original))
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	cookies := res.Cookies()
	if len(cookies) > 0 {
		return cookies[0]
	}
	return nil
}

func warmUpJSON(r http.Handler, original string) *http.Cookie {
	body, _ := json.Marshal(map[string]string{"url": original})

	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	cookies := res.Cookies()
	if len(cookies) > 0 {
		return cookies[0]
	}
	return nil
}

func createShortID(r http.Handler, original string) (string, error) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(original))
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	short := strings.TrimSpace(string(b))
	parts := strings.Split(short, "/")
	return parts[len(parts)-1], nil
}

func BenchmarkPOST_Plain(b *testing.B) {
	r, _ := newTestServer()
	original := "http://example.com/very/long/url"

	cookie := warmUpPlain(r, original) // прогрев: дальше пойдёт быстрый повтор (обычно 409 Conflict)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(original))
		req.Header.Set("Content-Type", "text/plain")
		if cookie != nil {
			req.AddCookie(cookie)
		}

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		_ = w.Result().Body.Close()
	}
}

func BenchmarkPOST_JSON(b *testing.B) {
	r, _ := newTestServer()
	original := "http://example.com/very/long/url"

	cookie := warmUpJSON(r, original)
	payload, _ := json.Marshal(map[string]string{"url": original})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		_ = w.Result().Body.Close()
	}
}

func BenchmarkGET_Redirect(b *testing.B) {
	r, _ := newTestServer()

	id, err := createShortID(r, "http://example.com")
	if err != nil {
		b.Fatalf("create id: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/"+id, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		_ = w.Result().Body.Close()
	}
}
