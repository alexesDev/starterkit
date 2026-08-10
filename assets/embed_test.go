package assets

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSPAHandlerResolvesBasePathInShell(t *testing.T) {
	handler := SPAHandler("/_system/starterkit", "abc/123")

	for _, path := range []string{"/", "/some/deep/link"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `<script src="/_system/starterkit/settings.js"></script>`)
		assert.Contains(t, w.Body.String(), `<link rel="stylesheet" href="/ui/app/-/web.css?rev=abc%2F123">`)
		assert.Contains(t, w.Body.String(), `<script src="/ui/app/-/web.js?rev=abc%2F123"></script>`)
		assert.NotContains(t, w.Body.String(), "{{base_path}}")
		assert.NotContains(t, w.Body.String(), "{{revision}}")
	}
}
