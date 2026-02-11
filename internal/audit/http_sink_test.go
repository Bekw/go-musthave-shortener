package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPSink_POSTsJSON(t *testing.T) {
	var got Event

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content-type: %s", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewHTTPSink(srv.URL, nil)
	e := Event{TS: 2, Action: "follow", UserID: "u2", URL: "https://ya.ru"}

	if err := s.Write(context.Background(), e); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got.TS != e.TS || got.Action != e.Action || got.UserID != e.UserID || got.URL != e.URL {
		t.Fatalf("unexpected event: %#v", got)
	}
}
