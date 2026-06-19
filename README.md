# template-static-site

An **Onklave project template** for static websites.

A clean [Vite](https://vitejs.dev/) + vanilla TypeScript starter. There is no
server and no runtime — the build produces a folder of static assets that
Onklave serves directly from the edge (no Dockerfile, no port).

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

`dist/` is the deploy artifact. Onklave serves it statically from the edge —
there is nothing to run, no container, and no listening port.

## Project structure

```
index.html        Vite entry / page markup
src/main.ts        Entry script (imports the stylesheet)
src/style.css      Styles
vite.config.ts     Build config (outputs to dist/)
tsconfig.json      TypeScript config
```
