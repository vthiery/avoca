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

type mockRetrier struct {
	maxAttempts int
}

var (
	errFailRequest    = errors.New("fail this request")
	dummyRequestBody  = `{ "id": "me" }`       // nolint
	dummyResponseBody = `{ "response": "ok" }` // nolint
	dummyHeader       = http.Header{           // nolint
		"content-type": {"application/json"},
	}
)

const dummyURL = "whatever"

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
	hardFailures   int
	beforeStatusOK int
	count          int
}

func (c *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c.count < c.hardFailures {
		c.count++
		return nil, errFailRequest
	}
	if c.count < c.beforeStatusOK {
		c.count++
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       ioutil.NopCloser(strings.NewReader(`{ "response": "not ok" }`)),
		}, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(strings.NewReader(`{ "response": "ok" }`)),
	}, nil
}

func (c *mockHTTPClient) reset() {
	c.count = 0
}

func failingHTTPClient(hardFailures int, beforeStatusOK int) *mockHTTPClient {
	return &mockHTTPClient{
		hardFailures:   hardFailures,
		beforeStatusOK: beforeStatusOK,
	}
}

func TestClientDoSuccess(t *testing.T) {
	hardFailures := 3
	beforeStatusOK := hardFailures + 1
	c := NewClient(
		WithHTTPClient(
			failingHTTPClient(hardFailures, beforeStatusOK),
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

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "whatever", nil)
	require.NoError(t, err)

	res, err := c.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
}

func TestClientDoFailure(t *testing.T) {
	hardFailures := 1
	beforeStatusOK := hardFailures + 1
	c := NewClient(
		WithHTTPClient(
			failingHTTPClient(hardFailures, beforeStatusOK),
		),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "whatever", nil)
	require.NoError(t, err)

	res, err := c.Do(req) // nolint
	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestClientSpecificMethodSuccess(t *testing.T) { // nolint: funlen
	hardFailures := 3
	beforeStatusOK := hardFailures + 1
	internalClient := failingHTTPClient(hardFailures, beforeStatusOK)

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
		name string
		fn   func() (*http.Response, error)
	}{
		{
			"Get",
			func() (*http.Response, error) {
				return c.Get(ctx, dummyURL, nil)
			},
		},
		{
			"Post",
			func() (*http.Response, error) {
				return c.Post(ctx, dummyURL, nil, nil)
			},
		},
		{
			"Put",
			func() (*http.Response, error) {
				return c.Put(ctx, dummyURL, nil, nil)
			},
		},
		{
			"Patch",
			func() (*http.Response, error) {
				return c.Patch(ctx, dummyURL, nil, nil)
			},
		},
		{
			"Delete",
			func() (*http.Response, error) {
				return c.Delete(ctx, dummyURL, nil)
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			res, err := tc.fn()
			require.NoError(t, err)
			defer res.Body.Close()

			assert.Equal(t, http.StatusOK, res.StatusCode)

			internalClient.reset()
		})
	}
}

func TestClientSpecificMethodHardFailure(t *testing.T) {
	hardFailures := 1
	beforeStatusOK := 1
	internalClient := failingHTTPClient(hardFailures, beforeStatusOK)

	c := NewClient(
		WithHTTPClient(internalClient),
	)

	ctx := context.Background()

	testCases := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		{
			"Get",
			func() (*http.Response, error) {
				return c.Get(ctx, dummyURL, nil)
			},
		},
		{
			"Post",
			func() (*http.Response, error) {
				return c.Post(ctx, dummyURL, nil, nil)
			},
		},
		{
			"Put",
			func() (*http.Response, error) {
				return c.Put(ctx, dummyURL, nil, nil)
			},
		},
		{
			"Patch",
			func() (*http.Response, error) {
				return c.Patch(ctx, dummyURL, nil, nil)
			},
		},
		{
			"Delete",
			func() (*http.Response, error) {
				return c.Delete(ctx, dummyURL, nil)
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			res, err := tc.fn() // nolint
			assert.Error(t, err)
			assert.Nil(t, res)

			internalClient.reset()
		})
	}
}

func TestClientSpecificMethodStatusFailure(t *testing.T) { // nolint
	hardFailures := 0
	beforeStatusOK := 3
	internalClient := failingHTTPClient(hardFailures, beforeStatusOK)

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
		name string
		fn   func() (*http.Response, error)
	}{
		{
			"Get",
			func() (*http.Response, error) {
				return c.Get(ctx, dummyURL, nil)
			},
		},
		{
			"Post",
			func() (*http.Response, error) {
				return c.Post(ctx, dummyURL, nil, nil)
			},
		},
		{
			"Put",
			func() (*http.Response, error) {
				return c.Put(ctx, dummyURL, nil, nil)
			},
		},
		{
			"Patch",
			func() (*http.Response, error) {
				return c.Patch(ctx, dummyURL, nil, nil)
			},
		},
		{
			"Delete",
			func() (*http.Response, error) {
				return c.Delete(ctx, dummyURL, nil)
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
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
		name string
		fn   func() (*http.Response, error)
	}{
		{
			"Get",
			func() (*http.Response, error) {
				return c.Get(nil, dummyURL, nil) // nolint
			},
		},
		{
			"Post",
			func() (*http.Response, error) {
				return c.Post(nil, dummyURL, nil, nil) // nolint
			},
		},
		{
			"Put",
			func() (*http.Response, error) {
				return c.Put(nil, dummyURL, nil, nil) // nolint
			},
		},
		{
			"Patch",
			func() (*http.Response, error) {
				return c.Patch(nil, dummyURL, nil, nil) // nolint
			},
		},
		{
			"Delete",
			func() (*http.Response, error) {
				return c.Delete(nil, dummyURL, nil) // nolint
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			res, err := tc.fn() // nolint
			assert.Error(t, err)
			assert.Nil(t, res)
		})
	}
}

type mockHTTPClientChecker struct {
	t *testing.T

	expectedMethod  string
	expectedURL     string
	expectedHeaders http.Header
	expectedBody    *string
}

func (c *mockHTTPClientChecker) Do(req *http.Request) (*http.Response, error) {
	assert.Equal(c.t, c.expectedMethod, req.Method)
	assert.Equal(c.t, c.expectedURL, req.URL.String())
	assert.Equal(c.t, c.expectedHeaders, req.Header)

	if c.expectedBody != nil {
		body, err := ioutil.ReadAll(req.Body)
		assert.NoError(c.t, err)
		assert.Equal(c.t, *c.expectedBody, string(body))
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(strings.NewReader(`{ "response": "ok" }`)),
	}, nil
}

func TestClientGet(t *testing.T) {
	c := NewClient(WithHTTPClient(&mockHTTPClientChecker{
		t,
		http.MethodGet,
		dummyURL,
		dummyHeader,
		nil,
	}))

	res, err := c.Get(context.Background(), dummyURL, dummyHeader)
	assert.NoError(t, res.Body.Close())
	assert.NoError(t, err)
}

func TestClientPost(t *testing.T) {
	c := NewClient(WithHTTPClient(&mockHTTPClientChecker{
		t,
		http.MethodPost,
		dummyURL,
		dummyHeader,
		&dummyRequestBody,
	}))

	res, err := c.Post(context.Background(), dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
	assert.NoError(t, res.Body.Close())
	assert.NoError(t, err)
}

func TestClientPut(t *testing.T) {
	c := NewClient(WithHTTPClient(&mockHTTPClientChecker{
		t,
		http.MethodPut,
		dummyURL,
		dummyHeader,
		&dummyRequestBody,
	}))

	res, err := c.Put(context.Background(), dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
	assert.NoError(t, res.Body.Close())
	assert.NoError(t, err)
}

func TestClientPatch(t *testing.T) {
	c := NewClient(WithHTTPClient(&mockHTTPClientChecker{
		t,
		http.MethodPatch,
		dummyURL,
		dummyHeader,
		&dummyRequestBody,
	}))

	res, err := c.Patch(context.Background(), dummyURL, ioutil.NopCloser(strings.NewReader(dummyRequestBody)), dummyHeader)
	assert.NoError(t, res.Body.Close())
	assert.NoError(t, err)
}

func TestClientDelete(t *testing.T) {
	c := NewClient(WithHTTPClient(&mockHTTPClientChecker{
		t,
		http.MethodDelete,
		dummyURL,
		dummyHeader,
		nil,
	}))

	res, err := c.Delete(context.Background(), dummyURL, dummyHeader)
	assert.NoError(t, res.Body.Close())
	assert.NoError(t, err)
}
