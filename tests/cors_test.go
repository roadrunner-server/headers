package headers

import (
	"net/http"
	"testing"

	"tests/helpers"

	"github.com/stretchr/testify/require"
)

const exposedHeaders = "Cache-Control, Content-Language, Content-Type, Expires, Last-Modified, Pragma"

// preflight sends the OPTIONS request a browser would send before a cross-origin
// call.
func preflight(t *testing.T, url string) response {
	t.Helper()

	return do(t, http.MethodOptions, url, map[string]string{
		"Access-Control-Request-Method":  http.MethodGet,
		"Access-Control-Request-Headers": "origin, x-requested-with",
		"Origin":                         requestOrig,
	})
}

// assertPreflight covers the headers a preflight response must carry. expOrigin
// differs between the wildcard and the regex config: the wildcard config echoes
// "*", the regex config echoes the caller's origin back.
func assertPreflight(t *testing.T, resp response, expOrigin string) {
	t.Helper()

	require.Equal(t, http.StatusOK, resp.status)
	require.Equal(t, "true", resp.header.Get("Access-Control-Allow-Credentials"))
	require.Equal(t, "origin, x-requested-with", resp.header.Get("Access-Control-Allow-Headers"))
	require.Equal(t, http.MethodGet, resp.header.Get("Access-Control-Allow-Methods"))
	require.Equal(t, expOrigin, resp.header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, "600", resp.header.Get("Access-Control-Max-Age"))
}

// assertActualRequest covers the headers the response to the real (non-preflight)
// cross-origin request must carry.
func assertActualRequest(t *testing.T, resp response, expOrigin string) {
	t.Helper()

	require.Equal(t, http.StatusOK, resp.status)
	require.Equal(t, "true", resp.header.Get("Access-Control-Allow-Credentials"))
	require.Equal(t, expOrigin, resp.header.Get("Access-Control-Allow-Origin"))
	require.Equal(t, exposedHeaders, resp.header.Get("Access-Control-Expose-Headers"))
}

// TestCORSWildcardOrigin uses allowed_origin "*", so both the preflight and the
// actual request echo "*" back.
func TestCORSWildcardOrigin(t *testing.T) {
	helpers.Start(t, "configs/.rr-cors-headers.yaml", headersPlugins(), helpers.WithTCPProbe(corsAddr))

	assertPreflight(t, preflight(t, "http://"+corsAddr), "*")
	assertActualRequest(t, get(t, "http://"+corsAddr, map[string]string{"Origin": requestOrig}), "*")
}

// TestCORSOriginRegex uses allowed_origin_regex, which matches the caller's
// origin and echoes it back rather than a wildcard.
func TestCORSOriginRegex(t *testing.T) {
	helpers.Start(t, "configs/.rr-cors-headers-regex.yaml", headersPlugins(), helpers.WithTCPProbe(corsAddr))

	assertPreflight(t, preflight(t, "http://"+corsAddr), requestOrig)
	assertActualRequest(t, get(t, "http://"+corsAddr, map[string]string{"Origin": requestOrig}), requestOrig)
}

// TestCORSOriginRegexRejectsNonMatchingOrigin sends an origin the regex does not
// cover, so no allow-origin header comes back and a browser would block it.
func TestCORSOriginRegexRejectsNonMatchingOrigin(t *testing.T) {
	helpers.Start(t, "configs/.rr-cors-headers-regex.yaml", headersPlugins(), helpers.WithTCPProbe(corsAddr))

	resp := get(t, "http://"+corsAddr, map[string]string{"Origin": "http://evil.example.com"})

	require.Empty(t, resp.header.Get("Access-Control-Allow-Origin"))
}
