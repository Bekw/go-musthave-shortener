package audit

import (
	"context"
	"time"

	"github.com/go-resty/resty/v2"
)

type HTTPSink struct {
	client *resty.Client
	url    string
}

func NewHTTPSink(url string, client *resty.Client) *HTTPSink {
	if url == "" {
		return nil
	}

	if client == nil {
		client = resty.New().
			SetTimeout(5 * time.Second).
			SetRetryCount(3).
			SetRetryWaitTime(1 * time.Second).
			SetRetryMaxWaitTime(5 * time.Second)
	}

	return &HTTPSink{
		client: client,
		url:    url,
	}
}

func (s *HTTPSink) Write(ctx context.Context, e Event) error {
	if s == nil || s.url == "" {
		return nil
	}

	_, err := s.client.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(e).
		Post(s.url)

	return err
}
