package audit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileSink_AppendsJSONLine(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "audit.log")

	s := NewFileSink(p)
	if s == nil {
		t.Fatal("sink is nil")
	}

	e := Event{TS: 1, Action: "shorten", UserID: "u1", URL: "https://example.com"}
	if err := s.Write(context.Background(), e); err != nil {
		t.Fatalf("write: %v", err)
	}

	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var got Event
	if err := json.Unmarshal([]byte(lines[0]), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.TS != e.TS || got.Action != e.Action || got.UserID != e.UserID || got.URL != e.URL {
		t.Fatalf("unexpected event: %#v", got)
	}
}
