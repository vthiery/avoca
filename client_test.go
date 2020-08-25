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
	errFailAttempt = errors.New("fail")
	errFailRequest = errors.New("fail this request")
)

const url = "whatever"

func (r *mockRetrier) Do(ctx context.Context, fn func() error) error {
	for attempt := 0; attempt < r.maxAttempts; attempt++ {
		if err := fn(); err != nil {
			continue
		}
		return nil
	}
	return errFailAttempt
}

type mockHTTPClient struct {
	failures int
	count    int
}

func (c *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if c.count < c.failures {
		c.count++
		return nil, errFailRequest
	}
	res := &http.Response{
		StatusCode: http.StatusOK,
	}
	res.Body = ioutil.NopCloser(strings.NewReader(`{ "response": "ok" }`))
	return res, nil
}

func (c *mockHTTPClient) reset() {
	c.count = 0
}

func failingHTTPClient(failures int) *mockHTTPClient {
	return &mockHTTPClient{
		failures: failures,
	}
}

func TestClientDoSuccess(t *testing.T) {
	failures := 3
	c := NewClient(
		WithHTTPClient(
			failingHTTPClient(failures),
		),
		WithRetrier(
			&mockRetrier{
				maxAttempts: failures + 1,
			},
		),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "whatever", nil)
	require.NoError(t, err)

	res, err := c.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	assert.Equal(t, http.StatusOK, res.StatusCode)
}

func TestClientDoFailure(t *testing.T) {
	failures := 1
	c := NewClient(
		WithHTTPClient(
			failingHTTPClient(failures),
		),
	)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "whatever", nil)
	require.NoError(t, err)

	res, err := c.Do(req) // nolint
	assert.Error(t, err)
	assert.Nil(t, res)
}

func TestClientSpecificMethodSuccess(t *testing.T) { // nolint: funlen
	failures := 3
	internalClient := failingHTTPClient(failures)

	c := NewClient(
		WithHTTPClient(internalClient),
		WithRetrier(
			&mockRetrier{
				maxAttempts: failures + 1,
			},
		),
	)

	ctx := context.Background()

	testCases := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		{
			"Get",
			func() (*http.Response, error) {
				return c.Get(ctx, url, nil)
			},
		},
		{
			"Post",
			func() (*http.Response, error) {
				return c.Post(ctx, url, nil, nil)
			},
		},
		{
			"Put",
			func() (*http.Response, error) {
				return c.Put(ctx, url, nil, nil)
			},
		},
		{
			"Patch",
			func() (*http.Response, error) {
				return c.Patch(ctx, url, nil, nil)
			},
		},
		{
			"Delete",
			func() (*http.Response, error) {
				return c.Delete(ctx, url, nil)
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

func TestClientSpecificMethodFailure(t *testing.T) {
	failures := 1
	internalClient := failingHTTPClient(failures)

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
				return c.Get(ctx, url, nil)
			},
		},
		{
			"Post",
			func() (*http.Response, error) {
				return c.Post(ctx, url, nil, nil)
			},
		},
		{
			"Put",
			func() (*http.Response, error) {
				return c.Put(ctx, url, nil, nil)
			},
		},
		{
			"Patch",
			func() (*http.Response, error) {
				return c.Patch(ctx, url, nil, nil)
			},
		},
		{
			"Delete",
			func() (*http.Response, error) {
				return c.Delete(ctx, url, nil)
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

func TestClientSpecificMethodBadContext(t *testing.T) {
	failures := 1
	c := NewClient(
		WithHTTPClient(
			failingHTTPClient(failures),
		),
	)

	testCases := []struct {
		name string
		fn   func() (*http.Response, error)
	}{
		{
			"Get",
			func() (*http.Response, error) {
				return c.Get(nil, url, nil) // nolint
			},
		},
		{
			"Post",
			func() (*http.Response, error) {
				return c.Post(nil, url, nil, nil) // nolint
			},
		},
		{
			"Put",
			func() (*http.Response, error) {
				return c.Put(nil, url, nil, nil) // nolint
			},
		},
		{
			"Patch",
			func() (*http.Response, error) {
				return c.Patch(nil, url, nil, nil) // nolint
			},
		},
		{
			"Delete",
			func() (*http.Response, error) {
				return c.Delete(nil, url, nil) // nolint
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
