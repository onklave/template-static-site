// Minimal static file server for the built site.
//
// Serves the assets in /www with correct MIME types (Go's mime package maps by
// extension, so ES-module .js is served as text/javascript) and answers /health
// for the platform's readiness/liveness probe. It writes nothing, so it runs
// happily as a non-root user on a read-only root filesystem.
package main

import (
	"net/http"
	"os"
	"strings"
	"time"
)

const root = "/www"

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: newHandler(http.Dir(root)),
		// A stalled client must not be able to hold a connection open forever
		// (Slowloris). There is deliberately no WriteTimeout: a static site can
		// serve large assets, and a phone on a slow link is a legitimate long
		// write that a timeout would truncate mid-download. API templates bound
		// their response writes; a file server cannot.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		panic(err)
	}
}

// newHandler builds the serving stack over a file system root. Taking the root
// as an argument is what makes the behaviour below testable.
func newHandler(fsys http.FileSystem) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", http.FileServer(indexOnlyDirs{fsys}))
	return securityHeaders(mux)
}

// securityHeaders sets the headers that are safe for any static site.
//
// Deliberately NOT set here: X-Frame-Options / frame-ancestors, because
// embedding a generated site in another page is a legitimate use of this
// template; and Content-Security-Policy, because a useful one depends on where
// YOUR app loads content from. Both are decisions for the app, not defaults for
// it — add them here once you know your embedding and content-origin story.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The file server sets Content-Type from the extension; nosniff stops
		// a browser second-guessing it and executing an asset as script.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// indexOnlyDirs serves a directory only when it contains an index.html,
// suppressing Go's built-in "index of" listing.
//
// The whole build output is deployed verbatim — including anything Vite copies
// through from public/ — so a listing turns every unreferenced draft asset from
// "unlinked" into "discoverable".
type indexOnlyDirs struct{ fs http.FileSystem }

func (d indexOnlyDirs) Open(name string) (http.File, error) {
	f, err := d.fs.Open(name)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if info.IsDir() {
		index, err := d.fs.Open(strings.TrimSuffix(name, "/") + "/index.html")
		if err != nil {
			_ = f.Close()
			return nil, os.ErrNotExist
		}
		_ = index.Close()
	}
	return f, nil
}
