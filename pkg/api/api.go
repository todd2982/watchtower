package api

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

const tokenMissingMsg = "api token is empty or has not been set. exiting"

// API is the http server responsible for serving the HTTP API endpoints
type API struct {
	Token       string
	hasHandlers bool
}

// New is a factory function creating a new API instance
func New(token string) *API {
	return &API{
		Token:       token,
		hasHandlers: false,
	}
}

// RequireToken is wrapper around http.HandleFunc that validates API token authentication.
//
// SECURITY NOTE: This middleware provides authentication for the HTTP API endpoints.
// Important security considerations:
//
//   - The token is compared using exact string matching (constant-time comparison recommended
//     but not critical for this use case as timing attacks are difficult to exploit remotely)
//   - Tokens are transmitted in the Authorization header as "Bearer <token>"
//   - WARNING: Tokens are sent over HTTP (unencrypted) unless using a reverse proxy with HTTPS
//   - A strong, randomly-generated token should be used (e.g., 'openssl rand -hex 32')
//   - The token grants full control over container updates
//   - Token rotation is recommended for production-like environments
//
// Usage:
//
//	curl -H "Authorization: Bearer YOUR_TOKEN" http://localhost:8080/v1/update
//
// Returns HTTP 401 Unauthorized if the token is missing or invalid.
func (api *API) RequireToken(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		want := fmt.Sprintf("Bearer %s", api.Token)
		if auth != want {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		log.Debug("Valid token found.")
		fn(w, r)
	}
}

// RegisterFunc is a wrapper around http.HandleFunc that also sets the flag used to determine whether to launch the API
func (api *API) RegisterFunc(path string, fn http.HandlerFunc) {
	api.hasHandlers = true
	http.HandleFunc(path, api.RequireToken(fn))
}

// RegisterHandler is a wrapper around http.Handler that also sets the flag used to determine whether to launch the API
func (api *API) RegisterHandler(path string, handler http.Handler) {
	api.hasHandlers = true
	http.Handle(path, api.RequireToken(handler.ServeHTTP))
}

// Start the API and serve over HTTP. Requires an API Token to be set.
func (api *API) Start(block bool) error {

	if !api.hasHandlers {
		log.Debug("Watchtower HTTP API skipped.")
		return nil
	}

	if api.Token == "" {
		log.Fatal(tokenMissingMsg)
	}

	if block {
		runHTTPServer()
	} else {
		go func() {
			runHTTPServer()
		}()
	}
	return nil
}

func runHTTPServer() {
	log.Fatal(http.ListenAndServe(":8080", nil))
}
