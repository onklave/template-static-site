module staticserver

// Keep this in step with the builder image in the root Dockerfile
// (`golang:1.26-alpine`) — that image is the source of truth for the Go
// version this server is built with.
go 1.26
