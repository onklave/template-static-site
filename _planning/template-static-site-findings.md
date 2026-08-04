# Findings — `onklave/template-static-site`

Issues found by cloning the template and running the customer happy path (`gh repo clone` → `npm ci` → `npm run build`). Build works flawlessly (17 pkgs, 0 vuln, 201ms, clean `dist/`). These are the fixes to make. Each is written as a paste-ready work item for the **Platform Templates** Project.

Severity: **P1** = customer-facing confusion / correctness · **P2** = consistency · **P3** = enhancement.

---

### T1 (P1) — README contradicts the actual deploy model
The README states: *"There is no server and no runtime … **no Dockerfile, no port**. Onklave serves `dist/` from the edge."* But the repo ships a `Dockerfile` **and** `server/main.go` (a Go static server) with `EXPOSE 3000`, and that container is the real deploy path. A customer gets directly contradictory signals about how their site is served (edge-static vs container).
- **Fix:** rewrite the README to describe the *actual* runtime contract — a built `dist/` served by a tiny non-root Go server on port 3000 with a `/health` probe, packaged as a distroless read-only container. If pure edge-static is the intended future direction, say "today it ships as X" rather than asserting the opposite of what's in the repo.
- **AC:** No statement in the README contradicts the shipped `Dockerfile`/`server/`. A reader can correctly say how their built site is served. `npm run build` + the Docker build both still succeed.

### T2 (P1) — "Project structure" section hides the deploy machinery
The README's structure list documents only `index.html`, `src/main.ts`, `src/style.css`, `vite.config.ts`, `tsconfig.json` — it omits `server/`, `Dockerfile`, and `onklave.yaml` entirely. The customer can't see (or is surprised by) the files that actually ship and deploy their app.
- **Fix:** either list the full structure including `server/`, `Dockerfile`, `onklave.yaml`, with a one-line "managed for you — you rarely edit these" note, or add a short "How it deploys" subsection.
- **SUPERSEDED IN PART (2026-08-04):** this originally said to document `.github/workflows/`. That file has since been DELETED from this template and must not come back. The platform builds, tests and deploys this repo in-cluster and never reads GitHub Actions; its GitHub credential cannot even push workflow files. `onklave.yaml` is the deploy declaration — document that instead. See onklave-platform `_specs/PLAN-fullstack-app-support.md` §12.10.
- **AC:** structure documentation matches the actual repo contents.

### T3 (P2) — Go toolchain version drift
`Dockerfile` builds with `golang:1.26-alpine`; `server/go.mod` declares `go 1.23`.
- **Fix:** align the two on one intended Go version; add a brief comment noting the source of truth.
- **AC:** Dockerfile Go image and `go.mod` version are consistent; Docker build succeeds.

### T4 (P2) — Build does not typecheck
`tsconfig.json` sets `strict`, `noUnusedLocals`, `noUnusedParameters`, but `npm run build` is `vite build`, which does not run `tsc`. CI only runs `npm run build`. Type errors (and the unused-symbol rules) are never enforced.
- **Fix:** add a typecheck step — e.g. `"typecheck": "tsc --noEmit"` and either fold it into `build` (`tsc --noEmit && vite build`) or add it as a CI step.
- **AC:** a deliberate type error fails `npm run build` (or CI); happy path still passes.

### T5 (P3) — Generic identity with no "edit these first" guidance
Ships as `template-static-site`, title `"App Name · Onklave"`, placeholder copy, an arbitrary blue/purple accent (`#5b8cff`/`#9b6bff`), and a system font — with no signpost telling the customer what to change first.
- **Fix (judgment call — confirm direction):** add a short "Make it yours — edit these first" block (name in `package.json`, `<title>`/meta, hero copy, accent token), **and/or** align the neutral starter with Onklave design tokens so customer sites begin closer to on-brand. Decide whether the template should be brand-neutral (customer's brand) or lightly Onklave-flavored.
- **AC:** a new customer can identify the first 4–5 things to edit within 30 seconds of opening the repo.

---

**Rollout note:** templates are load-bearing — every new customer project inherits them, so changes here propagate widely. Gate this Project's edit policy a notch stricter and treat each fix as its own reviewed diff.
