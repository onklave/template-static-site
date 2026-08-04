# template-static-site

An **Onklave project template** for static websites.

A clean [Vite](https://vitejs.dev/) + vanilla TypeScript starter. The build
produces a folder of static assets, which the shipped `Dockerfile` serves from a
tiny Go static server on port 3000.

## Use this template

Create a new repository from this template (the "Use this template" button on
GitHub, or `gh repo create <name> --template onklave/template-static-site`),
then:

```bash
npm ci          # install dependencies
npm run dev     # start the dev server with hot reload
```

Open the printed local URL and edit `index.html`, `src/style.css`, and
`src/main.ts`.

## Build

```bash
npm run build   # bundle → dist/
npm run preview # serve the built dist/ locally to sanity-check
```

`dist/` is baked into the container image at build time. Onklave reads
`onklave.yaml` to build that image and deploy it — that manifest is where the
port (3000), health path (`/health`) and route are declared, and where you would
add a second service such as an API or a worker. There is no CI workflow;
GitHub Actions is not part of the deploy path.

## Project structure

```
index.html         Vite entry / page markup
src/main.ts        Entry script (imports the stylesheet)
src/style.css      Styles
vite.config.ts     Build config (outputs to dist/)
tsconfig.json      TypeScript config
onklave.yaml       Build + deploy manifest (port, health path, route)
Dockerfile         Builds dist/ and packages it with the Go server
server/main.go     Static file server + /health endpoint
```
