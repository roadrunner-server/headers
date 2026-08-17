package headers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/roadrunner-server/errors"
	"github.com/stretchr/testify/require"
)

// stubConfigurer hands Init a pre-built Config instead of decoding YAML, so the
// tests exercise the plugin's own option mapping rather than the config decoder.
type stubConfigurer struct {
	sections map[string]bool
	cfg      *Config
}

func (s *stubConfigurer) Has(name string) bool { return s.sections[name] }

func (s *stubConfigurer) UnmarshalKey(_ string, out any) error {
	p, ok := out.(**Config)
	if !ok {
		return errors.Str("unexpected target type")
	}
	*p = s.cfg
	return nil
}

func bothSections() map[string]bool {
	return map[string]bool{RootPluginName: true, configKey: true}
}

func TestInitDisabledWithoutHTTPSection(t *testing.T) {
	err := (&Plugin{}).Init(&stubConfigurer{sections: map[string]bool{}})

	require.Error(t, err)
	require.True(t, errors.Is(errors.Disabled, err))
}

func TestInitDisabledWithoutHeadersSection(t *testing.T) {
	err := (&Plugin{}).Init(&stubConfigurer{sections: map[string]bool{RootPluginName: true}})

	require.Error(t, err)
	require.True(t, errors.Is(errors.Disabled, err))
}

func TestInitWithoutCORSLeavesHandlerUnset(t *testing.T) {
	p := &Plugin{}
	require.NoError(t, p.Init(&stubConfigurer{sections: bothSections(), cfg: &Config{}}))

	require.Nil(t, p.cors)
	require.Nil(t, p.allowedOriginRegex)
	require.NotNil(t, p.prop)
}

func TestInitCompilesAllowedOriginRegex(t *testing.T) {
	p := &Plugin{}
	cfg := &Config{CORS: &CORSConfig{AllowedOriginRegex: `^https?://example\.com$`}}
	require.NoError(t, p.Init(&stubConfigurer{sections: bothSections(), cfg: cfg}))

	require.NotNil(t, p.allowedOriginRegex)
	require.True(t, p.allowedOriginRegex.MatchString("https://example.com"))
	require.False(t, p.allowedOriginRegex.MatchString("https://evil.com"))
}

func TestInitRejectsBadAllowedOriginRegex(t *testing.T) {
	cfg := &Config{CORS: &CORSConfig{AllowedOriginRegex: "("}}

	err := (&Plugin{}).Init(&stubConfigurer{sections: bothSections(), cfg: cfg})

	require.Error(t, err)
}

// TestInitDefaultsOptionsSuccessStatus covers the compatibility default: an
// unset options_success_status must still answer preflights with 200.
func TestInitDefaultsOptionsSuccessStatus(t *testing.T) {
	p := &Plugin{}
	cfg := &Config{CORS: &CORSConfig{AllowedOrigin: "*", AllowedMethods: "GET"}}
	require.NoError(t, p.Init(&stubConfigurer{sections: bothSections(), cfg: cfg}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)

	p.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Result().StatusCode)
}

func TestInitHonoursCustomOptionsSuccessStatus(t *testing.T) {
	p := &Plugin{}
	cfg := &Config{CORS: &CORSConfig{
		AllowedOrigin:        "*",
		AllowedMethods:       "GET",
		OptionsSuccessStatus: http.StatusNoContent,
	}}
	require.NoError(t, p.Init(&stubConfigurer{sections: bothSections(), cfg: cfg}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)

	p.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Result().StatusCode)
}

// TestMiddlewareAppliesConfiguredHeaders checks both directions in one pass: the
// request header must be visible to the downstream handler, the response header
// must reach the recorder.
func TestMiddlewareAppliesConfiguredHeaders(t *testing.T) {
	p := &Plugin{cfg: &Config{
		Request:  map[string]string{"Input": "custom-header"},
		Response: map[string]string{"Output": "output-header"},
	}}

	var seen string
	h := p.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Input")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))

	require.Equal(t, "custom-header", seen)
	require.Equal(t, "output-header", rec.Header().Get("Output"))
}

func TestName(t *testing.T) {
	require.Equal(t, PluginName, (&Plugin{}).Name())
}
