package headers

import (
	"io"
	"net/http"
	"testing"

	"tests/helpers"

	headersPlugin "github.com/roadrunner-server/headers/v6"
	httpPlugin "github.com/roadrunner-server/http/v6"
	"github.com/roadrunner-server/server/v6"
	"github.com/stretchr/testify/require"
)

const (
	initAddr    = "127.0.0.1:33453"
	reqAddr     = "127.0.0.1:22655"
	respAddr    = "127.0.0.1:22455"
	corsAddr    = "127.0.0.1:22855"
	requestOrig = "http://127.0.0.1:10"
)

func headersPlugins() []any {
	return []any{&server.Plugin{}, &httpPlugin.Plugin{}, &headersPlugin.Plugin{}}
}

// response is the part of an http.Response the tests assert on, captured after
// the body has been read and closed.
type response struct {
	status int
	header http.Header
	body   string
}

// do issues the request and drains the response so no body is left open.
func do(t *testing.T, method, url string, hdr map[string]string) response {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, url, nil)
	require.NoError(t, err)
	for k, v := range hdr {
		req.Header.Add(k, v)
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	defer func() { require.NoError(t, resp.Body.Close()) }()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return response{status: resp.StatusCode, header: resp.Header.Clone(), body: string(body)}
}

func get(t *testing.T, url string, hdr map[string]string) response {
	t.Helper()
	return do(t, http.MethodGet, url, hdr)
}

// TestBootsWithHeadersSection proves the plugin initializes and serves when the
// http.headers section is present.
func TestBootsWithHeadersSection(t *testing.T) {
	helpers.Start(t, "configs/.rr-headers-init.yaml", headersPlugins(), helpers.WithTCPProbe(initAddr))
}

// TestRequestHeaderReachesWorker relies on header.php echoing the "input"
// request header back uppercased, so the body proves the middleware injected it
// before the request reached PHP.
func TestRequestHeaderReachesWorker(t *testing.T) {
	helpers.Start(t, "configs/.rr-req-headers.yaml", headersPlugins(), helpers.WithTCPProbe(reqAddr))

	resp := get(t, "http://"+reqAddr+"?hello=value", nil)

	require.Equal(t, http.StatusOK, resp.status)
	require.Equal(t, "CUSTOM-HEADER", resp.body)
}

// TestResponseHeaderIsAdded checks the configured response header lands on the
// way out while the request header still reaches the worker.
func TestResponseHeaderIsAdded(t *testing.T) {
	helpers.Start(t, "configs/.rr-res-headers.yaml", headersPlugins(), helpers.WithTCPProbe(respAddr))

	resp := get(t, "http://"+respAddr+"?hello=value", nil)

	require.Equal(t, http.StatusOK, resp.status)
	require.Equal(t, "output-header", resp.header.Get("output"))
	require.Equal(t, "CUSTOM-HEADER", resp.body)
}
