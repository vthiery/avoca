package avoca

import (
	"context"
	"errors"
	"io/ioutil"
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
		"content-type": {"application/json"},
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
		body, err := ioutil.ReadAll(req.Body)
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
			Body:       ioutil.NopCloser(strings.NewReader(dummyResponseBody)),
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(strings.NewReader(dummyResponseBody)),
	}, nil
}

func (c *mockHTTPClient) reset() {
	c.count = 0
}

func newMockHTTPClient(t *testing.T, hardFailures int, beforeStatusOK int) *mockHTTPClient {
	return &mockHTTPClient{
		t: t,

		hardFailures:   hardFailures,
		beforeStatusOK: beforeStatusOK,
	}
}

func TestClientDoSuccess(t *testing.T) {
	hardFailures := 3
	beforeStatusOK := hardFailures + 1
	c := NewClient(
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

	res, err := c.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
}

func TestClientSpecificMethodSuccess(t *testing.T) { // nolint: funlen
	hardFailures := 3
	beforeStatusOK := hardFailures + 1
	internalClient := newMockHTTPClient(t, hardFailures, beforeStatusOK)

	c := NewClient(
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

	ctx := context.Background()

	testCases := []struct {
		method string
		fn     func() (*http.Response, error)
	}{
		// {
		// 	http.MethodGet,
		// 	func() (*http.Response, error) {
		// 		return c.Get(ctx, dummyURL, dummyHeader)
		// 	},
		// },
		{
			http.MethodPost,
			func() (*http.Response, error) {
				return c.Post(ctx, dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
			},
		},
		// {
		// 	http.MethodPut,
		// 	func() (*http.Response, error) {
		// 		return c.Put(ctx, dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
		// 	},
		// },
		// {
		// 	http.MethodPatch,
		// 	func() (*http.Response, error) {
		// 		return c.Patch(ctx, dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
		// 	},
		// },
		// {
		// 	http.MethodDelete,
		// 	func() (*http.Response, error) {
		// 		return c.Delete(ctx, dummyURL, dummyHeader)
		// 	},
		// },
	}
	for _, tc := range testCases {
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
	c := NewClient(
		WithHTTPClient(
			newMockHTTPClient(t, hardFailures, beforeStatusOK),
		),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, dummyURL, nil)
	require.NoError(t, err)

	req.Header = dummyHeader

	res, err := c.Do(req) // nolint
	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestClientSpecificMethodHardFailure(t *testing.T) {
	hardFailures := 1
	beforeStatusOK := 1
	internalClient := newMockHTTPClient(t, hardFailures, beforeStatusOK)

	c := NewClient(
		WithHTTPClient(internalClient),
	)

	ctx := context.Background()

	testCases := []struct {
		method string
		fn     func() (*http.Response, error)
	}{
		{
			http.MethodGet,
			func() (*http.Response, error) {
				return c.Get(ctx, dummyURL, dummyHeader)
			},
		},
		{
			http.MethodPost,
			func() (*http.Response, error) {
				return c.Post(ctx, dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
			},
		},
		{
			http.MethodPut,
			func() (*http.Response, error) {
				return c.Put(ctx, dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
			},
		},
		{
			http.MethodPatch,
			func() (*http.Response, error) {
				return c.Patch(ctx, dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
			},
		},
		{
			http.MethodDelete,
			func() (*http.Response, error) {
				return c.Delete(ctx, dummyURL, dummyHeader)
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.method, func(t *testing.T) {
			res, err := tc.fn() // nolint
			assert.Error(t, err)
			assert.Nil(t, res)

			internalClient.reset()
		})
	}
}

func TestClientDoStatusFailure(t *testing.T) {
	hardFailures := 0
	beforeStatusOK := 3
	c := NewClient(
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

	res, err := c.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
}

func TestClientSpecificMethodStatusFailure(t *testing.T) { // nolint
	hardFailures := 0
	beforeStatusOK := 3
	internalClient := newMockHTTPClient(t, hardFailures, beforeStatusOK)

	c := NewClient(
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

	ctx := context.Background()

	testCases := []struct {
		method string
		fn     func() (*http.Response, error)
	}{
		{
			http.MethodGet,
			func() (*http.Response, error) {
				return c.Get(ctx, dummyURL, dummyHeader)
			},
		},
		{
			http.MethodPost,
			func() (*http.Response, error) {
				return c.Post(ctx, dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
			},
		},
		{
			http.MethodPut,
			func() (*http.Response, error) {
				return c.Put(ctx, dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
			},
		},
		{
			http.MethodPatch,
			func() (*http.Response, error) {
				return c.Patch(ctx, dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
			},
		},
		{
			http.MethodDelete,
			func() (*http.Response, error) {
				return c.Delete(ctx, dummyURL, dummyHeader)
			},
		},
	}
	for _, tc := range testCases {
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

	testCases := []struct {
		method string
		fn     func() (*http.Response, error)
	}{
		{
			http.MethodGet,
			func() (*http.Response, error) {
				return c.Get(nil, dummyURL, dummyHeader) // nolint
			},
		},
		{
			http.MethodPost,
			func() (*http.Response, error) {
				return c.Post(nil, dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader) // nolint
			},
		},
		{
			http.MethodPut,
			func() (*http.Response, error) {
				return c.Put(nil, dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader) // nolint
			},
		},
		{
			http.MethodPatch,
			func() (*http.Response, error) {
				return c.Patch(nil, dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader) // nolint
			},
		},
		{
			http.MethodDelete,
			func() (*http.Response, error) {
				return c.Delete(nil, dummyURL, dummyHeader) // nolint
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.method, func(t *testing.T) {
			res, err := tc.fn() // nolint
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	}
}
