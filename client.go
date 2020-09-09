package avoca

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// Client composes a Doer and a Retrier.
// By default, the client uses:
//     * a http.Client with a timeout set to 60 * time.Second
//     * a retrier that does not retry
//     * a retry policy that return false for all HTTP codes
type Client struct {
	client      Doer
	retrier     Retrier
	retryPolicy RetryPolicy
}

// Doer interface that match the standard HTTP client `http.Do` interface.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Retrier interface that allows to perform retries.
type Retrier interface {
	Do(context.Context, func(context.Context) error) error
}

// RetryPolicy describes the retry policy based on HTTP codes.
// It should return true for retryable HTTP code and false otherwise.
type RetryPolicy func(statusCode int) bool

// Do makes an HTTP request with the native `http.Do` interface.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	var (
		res *http.Response
		err error
	)
	err = c.retrier.Do(req.Context(), func(context.Context) error {
		res, err = c.client.Do(req) // nolint
		if err != nil {
			return err
		}
		if c.retryPolicy(res.StatusCode) {
			// Return a statusError to try again
			return statusError
		}
		// The request went fine, no need to retry
		return nil
	})
	if err != nil && !errors.Is(err, statusError) {
		return nil, err
	}
	return res, nil
}

// Get makes a HTTP GET request to provided URL.
func (c *Client) Get(ctx context.Context, url string, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, &RequestCreationError{err}
	}
	req.Header = headers
	return c.Do(req)
}

// Post makes a HTTP POST request to provided URL.
func (c *Client) Post(ctx context.Context, url string, body io.Reader, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, &RequestCreationError{err}
	}
	req.Header = headers
	return c.Do(req)
}

// Put makes a HTTP PUT request to provided URL.
func (c *Client) Put(ctx context.Context, url string, body io.Reader, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, body)
	if err != nil {
		return nil, &RequestCreationError{err}
	}
	req.Header = headers
	return c.Do(req)
}

// Patch makes a HTTP PATCH request to provided URL.
func (c *Client) Patch(ctx context.Context, url string, body io.Reader, headers http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, body)
	if err != nil {
		return nil, &RequestCreationError{err}
	}
	req.Header = headers
	return c.Do(req)
}

// Delete makes a HTTP DELETE request to provided URL.
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

// WithRetryPolicy sets the retry policy.
func WithRetryPolicy(retryPolicy RetryPolicy) Option {
	return func(c *Client) {
		c.retryPolicy = retryPolicy
	}
}

// NewClient returns a new instance of the Client.
func NewClient(opts ...Option) *Client {
	client := Client{
		client: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		retrier:     &noRetry{},
		retryPolicy: defaultRetryPolicy,
	}
	for _, opt := range opts {
		opt(&client)
	}
	return &client
}

const defaultHTTPTimeout = 60 * time.Second

type noRetry struct{}

func (r *noRetry) Do(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func defaultRetryPolicy(statusCode int) bool {
	return false
}
