package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

func Example_postPlain() {
	r, base := newTestServer()

	original := "http://example.com/very/long/url"
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(original))
	req.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	b, _ := io.ReadAll(res.Body)
	short := strings.TrimSpace(string(b))

	fmt.Println(res.StatusCode, strings.HasPrefix(short, base+"/"))

	// Output:
	// 201 true
}

func Example_postJSON() {
	r, base := newTestServer()

	body := []byte(`{"url":"http://example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/shorten", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	res := w.Result()
	defer res.Body.Close()

	var got struct {
		Result string `json:"result"`
	}
	_ = json.NewDecoder(res.Body).Decode(&got)

	fmt.Println(res.StatusCode, strings.HasPrefix(got.Result, base+"/"))

	// Output:
	// 201 true
}

func Example_getRedirect() {
	r, base := newTestServer()

	original := "http://example.com/very/long/url"

	req1 := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(original))
	req1.Header.Set("Content-Type", "text/plain")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)

	res1 := w1.Result()
	defer res1.Body.Close()

	b1, _ := io.ReadAll(res1.Body)
	short := strings.TrimSpace(string(b1))
	id := strings.TrimPrefix(short, base+"/")

	req2 := httptest.NewRequest(http.MethodGet, "/"+id, nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	res2 := w2.Result()
	defer res2.Body.Close()

	fmt.Println(res2.StatusCode, res2.Header.Get("Location"))

	// Output:
	// 307 http://example.com/very/long/url
}
