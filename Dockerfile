# Build the Vite static site, then serve it from a tiny, dependency-free Go
# server. The platform runs tenant containers as a NON-ROOT user (uid 10001) on
# a READ-ONLY root filesystem, with an HTTP /health probe on port 3000. This
# image satisfies all three with no extra config — no nginx temp dirs, no
# writable paths, no privileged ports.

# 1. Build the static assets (-> /app/dist).
FROM node:22-alpine AS web
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

# 2. Build the static file server (static binary, no libc).
FROM golang:1.26-alpine AS server
WORKDIR /src
COPY server/go.mod ./
COPY server/main.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /server .

# 3. Minimal runtime: just the server binary + the built assets.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=web /app/dist /www
COPY --from=server /server /server
EXPOSE 3000
ENTRYPOINT ["/server"]
