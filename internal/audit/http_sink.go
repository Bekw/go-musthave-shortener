package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type HTTPSink struct {
	url    string
	client *http.Client
}

func NewHTTPSink(url string, c *http.Client) *HTTPSink {
	if url == "" {
		return nil
	}
	if c == nil {
		c = &http.Client{Timeout: 2 * time.Second}
	}
	return &HTTPSink{url: url, client: c}
}

func (s *HTTPSink) Write(ctx context.Context, e Event) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("audit receiver status %d", resp.StatusCode)
	}
	return nil
}
