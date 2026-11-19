package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/99designs/gqlgen/graphql/playground"

	"starterkit/assets"
	"starterkit/internal/appreq"
	"starterkit/internal/clientip"
	"starterkit/internal/graph"
)

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()
	base := a.config.BasePath

	mux.HandleFunc("GET "+base+"/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET "+base+"/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !a.ready.Load() {
			http.Error(w, "shutting down", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	api := graph.NewHandler(a, graph.Introspection(a.config.DevMode))
	mux.Handle("POST "+base+"/graphql", limitBody(a.requireIdentity(a.build.Announce(api))))

	if a.config.DevMode {
		mux.Handle("GET "+base+"/graphql", a.requireIdentity(playgroundPolicy(playground.Handler("Starterkit", base+"/graphql"))))
	}

	mux.Handle("GET "+base+"/settings.js", a.requireIdentity(settingsScript(base)))
	mux.Handle("GET /", a.requireIdentity(assets.SPAHandler(base, a.build.Revision())))

	root := http.NewServeMux()
	root.Handle(base+"/mcp", a.mcpRoute())
	root.Handle("/", a.withRequest(mux))

	return securityHeaders(http.NewCrossOriginProtection().Handler(root))
}

func settingsScript(basePath string) http.Handler {
	settings := struct {
		GraphQLURL string `json:"graphqlUrl"`
	}{
		GraphQLURL: basePath + "/graphql",
	}

	encoded, _ := json.Marshal(settings)
	script := append([]byte("window.starterkit_settings = "), encoded...)
	script = append(script, '\n')

	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		_, _ = w.Write(script)
	})
}

const contentSecurityPolicy = "default-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"frame-ancestors 'none'; object-src 'none'; base-uri 'none'"

func playgroundPolicy(next http.Handler) http.Handler {
	const policy = "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
		"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; img-src 'self' data: https://cdn.jsdelivr.net; " +
		"connect-src 'self'; frame-ancestors 'none'"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", policy)
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")

		next.ServeHTTP(w, r)
	})
}

const maxRequestBody = 1 << 20

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		next.ServeHTTP(w, r)
	})
}

func (a *app) withRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, r := a.newRequest(r)
		ctx := r.Context()

		identity, err := a.resolveIdentity(req, r)
		if err != nil {
			a.log.Error("failed to resolve identity", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)

			return
		}

		if identity.Banned() {
			http.Error(w, "access revoked: "+identity.BanReason, http.StatusForbidden)
			return
		}

		if identity.Stale() {
			a.refuseStaleToken(w, r)
			return
		}

		next.ServeHTTP(w, r)

		a.flushAudit(ctx, req.TakeAudit())
	})
}

func (a *app) newRequest(r *http.Request) (*appreq.Request, *http.Request) {
	req := appreq.New(clientip.Resolve(r, a.config.Proxies.Trusted))

	return req, r.WithContext(req.NewContext(r.Context()))
}

func (a *app) refuseStaleToken(w http.ResponseWriter, r *http.Request) {
	if isNavigation(r) {
		http.Redirect(w, r, a.config.SignOutURL, http.StatusSeeOther)
		return
	}

	http.Error(w, "the gate's token no longer verifies: sign in again", http.StatusUnauthorized)
}

const modeNavigate = "navigate"

func isNavigation(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}

	mode := r.Header.Get("Sec-Fetch-Mode")
	if mode != "" {
		return mode == modeNavigate
	}

	return strings.Contains(r.Header.Get("Accept"), "text/html")
}

func (a *app) requireIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := appreq.FromCtx(r.Context())
		if err != nil || req.UserID() == nil {
			http.Error(w, "no verified identity on this request: reach the panel through the gate", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
