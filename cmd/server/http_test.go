package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"starterkit/internal/appconfig"
	"starterkit/internal/gateauth"
)

func newRoutedApp(t *testing.T) http.Handler {
	t.Helper()

	a := newTestApp(t)
	a.config = &appconfig.Config{BasePath: "/_system/starterkit"}
	a.gate = &gateauth.Client{}

	return a.routes()
}

func postGraphQL(routes http.Handler, header http.Header) int {
	r := httptest.NewRequest(http.MethodPost, "http://panel.example/_system/starterkit/graphql", strings.NewReader(`{"query":"{viewer{role}}"}`))
	r.Header = header
	r.Header.Set("Content-Type", "text/plain")

	w := httptest.NewRecorder()
	routes.ServeHTTP(w, r)

	return w.Code
}

func TestACrossSitePostNeverReachesTheAPI(t *testing.T) {
	routes := newRoutedApp(t)

	crossSite := http.Header{"Sec-Fetch-Site": {"cross-site"}}
	require.Equal(t, http.StatusForbidden, postGraphQL(routes, crossSite))

	foreignOrigin := http.Header{"Origin": {"https://attacker.example"}}
	require.Equal(t, http.StatusForbidden, postGraphQL(routes, foreignOrigin))
}

func TestASameOriginPostPassesToTheIdentityGate(t *testing.T) {
	routes := newRoutedApp(t)

	sameOrigin := http.Header{"Sec-Fetch-Site": {"same-origin"}}
	require.Equal(t, http.StatusUnauthorized, postGraphQL(routes, sameOrigin), "refused by the identity gate, not by the origin check")

	noBrowser := http.Header{}
	require.Equal(t, http.StatusUnauthorized, postGraphQL(routes, noBrowser), "a client without browser headers is not a cross-site request")
}
