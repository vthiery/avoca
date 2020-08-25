package avoca

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	client  Doer
	retrier Retrier
}

type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

type Retrier interface {
	Do(context.Context, func() error) error
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
	var (
		res *http.Response
		err error
	)
	if err := c.retrier.Do(req.Context(), func() error {
		res, err = c.client.Do(req) // nolint
		return err
	}); err != nil {
		return nil, err
	}
	return res, nil
}

type RequestCreationError struct {
	Err error
}

func (e *RequestCreationError) Error() string {
	return fmt.Errorf("request creation failed: %w", e.Err).Error()
}

func (c *Client) Get(ctx context.Context, url string, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, &RequestCreationError{err}
	}
	req.Header = headers
	return c.Do(req)
}

func (c *Client) Post(ctx context.Context, url string, body io.Reader, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, &RequestCreationError{err}
	}
	req.Header = headers
	return c.Do(req)
}

func (c *Client) Put(ctx context.Context, url string, body io.Reader, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
	if err != nil {
		return nil, &RequestCreationError{err}
	}
	req.Header = headers
	return c.Do(req)
}

func (c *Client) Patch(ctx context.Context, url string, body io.Reader, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, nil)
	if err != nil {
		return nil, &RequestCreationError{err}
	}
	req.Header = headers
	return c.Do(req)
}

func (c *Client) Delete(ctx context.Context, url string, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return nil, &RequestCreationError{err}
	}
	req.Header = headers
	return c.Do(req)
}

// Option represents the client options.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client Doer) Option {
	return func(c *Client) {
		c.client = client
	}
}

// WithRetrier sets the strategy for retrying.
func WithRetrier(retrier Retrier) Option {
	return func(c *Client) {
		c.retrier = retrier
	}
}

const defaultHTTPTimeout = 60 * time.Second

// NewClient returns a new instance of the Client.
func NewClient(opts ...Option) *Client {
	client := Client{
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		retrier: &noRetry{},
	}
	for _, opt := range opts {
		opt(&client)
	}
	return &client
}

type noRetry struct{}

func (r *noRetry) Do(ctx context.Context, fn func() error) error {
	return fn()
}
