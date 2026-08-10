// Package assets embeds the shell and the built $mol bundle.
package assets

import (
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
)

//go:embed index.html
var shell embed.FS

//go:embed all:ui
var ui embed.FS

// SPAHandler serves the built bundle under /ui/ and falls back to the shell for
// everything else, so client-side routes survive a reload. The shell's
// placeholders are resolved once, here, so the settings script follows the
// configured prefix and static assets change URL on every deploy.
func SPAHandler(basePath, revision string) http.Handler {
	uiFS, err := fs.Sub(ui, "ui")
	if err != nil {
		panic(err)
	}

	files := http.FileServer(http.FS(uiFS))

	page, err := shell.ReadFile("index.html")
	if err != nil {
		panic(err)
	}

	page = bytes.ReplaceAll(page, []byte("{{base_path}}"), []byte(basePath))
	page = bytes.ReplaceAll(page, []byte("{{revision}}"), []byte(url.QueryEscape(revision)))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/ui/") {
			http.StripPrefix("/ui/", files).ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
}
