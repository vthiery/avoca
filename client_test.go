package avoca

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// nolint:gochecknoglobals
var (
	errFailRequest    = errors.New("fail this request")
	dummyURL          = `whatever`
	dummyRequestBody  = `{ "id": "me" }`
	dummyResponseBody = `{ "response": "whatever" }`
	dummyHeader       = http.Header{
		"content-type": []string{"application/json"},
	}
)

type mockRetrier struct {
	maxAttempts int
}

func (r *mockRetrier) Do(ctx context.Context, fn func(context.Context) error) error {
	var err error
	for attempt := 0; attempt < r.maxAttempts; attempt++ {
		if err = fn(ctx); err != nil {
			continue
		}

		return nil
	}

	return err
}

type mockHTTPClient struct {
	t *testing.T

	hardFailures   int
	beforeStatusOK int
	count          int
}

func (c *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	// Sanity checks
	assert.Equal(c.t, dummyURL, req.URL.String())
	assert.Equal(c.t, dummyHeader, req.Header)

	if req.Body != nil {
		body, err := io.ReadAll(req.Body)
		assert.NoError(c.t, err)
		assert.Equal(c.t, dummyRequestBody, string(body))
	}

	if c.count < c.hardFailures {
		c.count++

		return nil, errFailRequest
	}
	if c.count < c.beforeStatusOK {
		c.count++

		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(strings.NewReader(dummyResponseBody)),
		}, nil
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(dummyResponseBody)),
	}, nil
}

func (c *mockHTTPClient) reset() {
	c.count = 0
}

func newMockHTTPClient(t *testing.T, hardFailures int, beforeStatusOK int) *mockHTTPClient {
	t.Helper()

	return &mockHTTPClient{
		t: t,

		hardFailures:   hardFailures,
		beforeStatusOK: beforeStatusOK,
	}
}

func TestClientDoSuccess(t *testing.T) {
	hardFailures := 3
	beforeStatusOK := hardFailures + 1
	client := NewClient(
		WithHTTPClient(
			newMockHTTPClient(t, hardFailures, beforeStatusOK),
		),
		WithRetrier(
			&mockRetrier{
				maxAttempts: beforeStatusOK + 1,
			},
		),
		WithRetryPolicy(func(statusCode int) bool {
			return statusCode >= http.StatusInternalServerError
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, dummyURL, nil)
	require.NoError(t, err)

	req.Header = dummyHeader

	res, err := client.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
}

func allMethodsTestCases(ctx context.Context, client *Client) []struct {
	method string
	fn     func() (*http.Response, error)
} {
	return []struct {
		method string
		fn     func() (*http.Response, error)
	}{
		{
			http.MethodGet,
			func() (*http.Response, error) {
				return client.Get(ctx, dummyURL, dummyHeader)
			},
		},
		{
			http.MethodPost,
			func() (*http.Response, error) {
				return client.Post(ctx, dummyURL, io.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
			},
		},
		{
			http.MethodPut,
			func() (*http.Response, error) {
				return client.Put(ctx, dummyURL, io.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
			},
		},
		{
			http.MethodPatch,
			func() (*http.Response, error) {
				return client.Patch(ctx, dummyURL, io.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
			},
		},
		{
			http.MethodDelete,
			func() (*http.Response, error) {
				return client.Delete(ctx, dummyURL, dummyHeader)
			},
		},
	}
}

func TestClientSpecificMethodSuccess(t *testing.T) {
	hardFailures := 3
	beforeStatusOK := hardFailures + 1
	internalClient := newMockHTTPClient(t, hardFailures, beforeStatusOK)

	client := NewClient(
		WithHTTPClient(internalClient),
		WithRetrier(
			&mockRetrier{
				maxAttempts: beforeStatusOK + 1,
			},
		),
		WithRetryPolicy(func(statusCode int) bool {
			return statusCode >= http.StatusInternalServerError
		}),
	)

	for _, tc := range allMethodsTestCases(context.Background(), client) {
		tc := tc
		t.Run(tc.method, func(t *testing.T) {
			res, err := tc.fn()
			require.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, http.StatusOK, res.StatusCode)

			internalClient.reset()
		})
	}
}

func TestClientDoHardFailure(t *testing.T) {
	hardFailures := 1
	beforeStatusOK := hardFailures + 1
	client := NewClient(
		WithHTTPClient(
			newMockHTTPClient(t, hardFailures, beforeStatusOK),
		),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, dummyURL, nil)
	require.NoError(t, err)

	req.Header = dummyHeader

	res, err := client.Do(req)
	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestClientSpecificMethodHardFailure(t *testing.T) {
	hardFailures := 1
	beforeStatusOK := 1
	internalClient := newMockHTTPClient(t, hardFailures, beforeStatusOK)

	client := NewClient(
		WithHTTPClient(internalClient),
	)

	for _, tc := range allMethodsTestCases(context.Background(), client) {
		tc := tc
		t.Run(tc.method, func(t *testing.T) {
			res, err := tc.fn()
			assert.Error(t, err)
			assert.Nil(t, res)

			internalClient.reset()
		})
	}
}

func TestClientDoStatusFailure(t *testing.T) {
	hardFailures := 0
	beforeStatusOK := 3
	client := NewClient(
		WithHTTPClient(
			newMockHTTPClient(t, hardFailures, beforeStatusOK),
		),
		WithRetrier(
			&mockRetrier{
				maxAttempts: 1,
			},
		),
		WithRetryPolicy(func(statusCode int) bool {
			return statusCode >= http.StatusInternalServerError
		}),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, dummyURL, nil)
	require.NoError(t, err)

	req.Header = dummyHeader

	res, err := client.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
}

func TestClientSpecificMethodStatusFailure(t *testing.T) {
	hardFailures := 0
	beforeStatusOK := 3
	internalClient := newMockHTTPClient(t, hardFailures, beforeStatusOK)

	client := NewClient(
		WithHTTPClient(internalClient),
		WithRetrier(
			&mockRetrier{
				maxAttempts: 1,
			},
		),
		WithRetryPolicy(func(statusCode int) bool {
			return statusCode >= http.StatusInternalServerError
		}),
	)

	for _, tc := range allMethodsTestCases(context.Background(), client) {
		tc := tc
		t.Run(tc.method, func(t *testing.T) {
			res, err := tc.fn()
			require.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, http.StatusInternalServerError, res.StatusCode)

			internalClient.reset()
		})
	}
}

func TestClientSpecificMethodBadContext(t *testing.T) {
	c := NewClient()

	for _, tc := range allMethodsTestCases(nil, c) {
		tc := tc
		t.Run(tc.method, func(t *testing.T) {
			res, err := tc.fn()
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	}
}

func TestCopyHTTPRequestBody(t *testing.T) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		dummyURL,
		io.NopCloser(strings.NewReader(dummyRequestBody)),
	)
	assert.NoError(t, err)

	body, err := copyHTTPRequestBody(req)
	assert.NoError(t, err)
	assert.Equal(t, dummyRequestBody, string(body))
}

func TestCopyHTTPRequestBodyNil(t *testing.T) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, dummyURL, nil)
	assert.NoError(t, err)

	body, err := copyHTTPRequestBody(req)
	assert.NoError(t, err)
	assert.Nil(t, body)
}

type brokenCloser struct {
	io.Reader
}

func (brokenCloser) Close() error {
	return errors.New("cannot close")
}

func TestCopyHTTPRequestBodyFailCloseOriginal(t *testing.T) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		dummyURL,
		brokenCloser{strings.NewReader(dummyRequestBody)},
	)
	assert.NoError(t, err)

	body, err := copyHTTPRequestBody(req)
	assert.Error(t, err)
	assert.Nil(t, body)
}

func TestNewNopCloserFromBody(t *testing.T) {
	rc := newNopCloserFromBody([]byte(dummyRequestBody))

	assert.NotNil(t, rc)

	body, err := io.ReadAll(rc)
	assert.NoError(t, err)
	assert.Equal(t, dummyRequestBody, string(body))
}

func TestNewNopCloserFromBodyNil(t *testing.T) {
	assert.Nil(t, newNopCloserFromBody(nil))
}

func TestDefaultRetryPolicy(t *testing.T) {
	n := rand.Int() // nolint:gosec
	assert.False(t, defaultRetryPolicy(n))
}
