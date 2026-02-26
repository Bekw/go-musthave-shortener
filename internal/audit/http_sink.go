package audit

import (
	"context"
	"time"

	"github.com/go-resty/resty/v2"
)

const (
	defaultHTTPTimeout      = 5 * time.Second
	defaultRetryCount       = 3
	defaultRetryWaitTime    = 1 * time.Second
	defaultRetryMaxWaitTime = 5 * time.Second
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
			SetTimeout(defaultHTTPTimeout).
			SetRetryCount(defaultRetryCount).
			SetRetryWaitTime(defaultRetryWaitTime).
			SetRetryMaxWaitTime(defaultRetryMaxWaitTime)
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
