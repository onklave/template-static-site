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
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", http.FileServer(http.Dir("/www")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		panic(err)
	}
}
