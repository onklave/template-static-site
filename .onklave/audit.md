# Template audit

- **Last audited:** 2026-08-04
- **Audited by:** Onklave platform maintenance (automated, Claude Code)
- **Next review due:** 2026-11-04 (quarterly, or sooner on a dependency alert)

## Why this file exists
So we know when this template was last deliberately checked, and what was true at
the time. Apps are generated from this repo — stale or vulnerable dependencies
here propagate to every app created from it.

## Scope of this audit
- Full clean install and build of the Vite/TypeScript frontend from the committed
  lockfile (`npm ci`, not a warm `node_modules`).
- npm advisory and outdated-package status, and the upgrades that follow from them.
- The Go static server in `server/`: toolchain version, `go vet`, `go test`,
  `go mod tidy`, a production-flag build, and `govulncheck`.
- `Dockerfile` security review: base-image currency, non-root execution,
  read-only-root-filesystem compatibility, build-context hygiene.
- A real `docker build` plus a runtime smoke test of the resulting image under the
  constraints the platform applies (read-only rootfs, dropped capabilities,
  uid 10001).
- A secret scan across every tracked file and the full git history.

Not in scope / deliberately untouched: `onklave.yaml` (written and verified in a
previous pass), page copy and styling, and the README.

Tooling note: the audit machine has no local Go toolchain. Every Go command was
run inside `golang:1.26-alpine` — the same builder image the Dockerfile uses —
with `server/` bind-mounted, so the results reflect the real build toolchain.

## Verification run
| Check | Command | Result |
|---|---|---|
| Clean install | `rm -rf node_modules && npm ci` | Pass — 18 packages, 0 vulnerabilities |
| Frontend build | `npm run build` | Pass — 5 modules transformed, `dist/` written, 196 ms |
| Frontend tests | `npm test` | **N/A** — no `test` script exists (`npm error Missing script: "test"`). See finding 9 |
| Typecheck | `npx tsc --noEmit` | Pass (exit 0) — **failed before this audit** with `TS2882` on `src/main.ts:1`; see finding 8 |
| Typecheck really enforces | added a deliberate type error, re-ran `npx tsc --noEmit` | Pass — exit 1 with `TS2322` + `TS6133`; error then reverted |
| npm advisories | `npm audit` | Pass — 0 vulnerabilities (was **1 high**; see finding 1) |
| Outdated packages | `npm outdated` | Clean — no outdated packages remain |
| Go vet | `go vet ./...` | Pass |
| Go tests | `go test ./...` | Pass — was `?   staticserver   [no test files]`; now `ok staticserver`, 4 tests (added in the reconciliation pass, see finding 4) |
| Go module tidy | `go mod tidy` | Pass — no change (stdlib only, no `go.sum`) |
| Go build | `CGO_ENABLED=0 go build -trimpath -o /server .` | Pass |
| Go vulnerabilities | `govulncheck ./...` (v1.6.0, installed via `go install golang.org/x/vuln/cmd/govulncheck@latest` in the builder container) | Pass — `No vulnerabilities found.` |
| Image build | `docker build --no-cache -t template-static-site:audit2 .` | Pass — final image 18.6 MB |
| Runtime smoke | `docker run --read-only --cap-drop=ALL --user 10001:10001 -p 38081:3000 …` | Pass — `/health` → `200 ok`; `/` → `200 text/html`; `/assets/*.js` → `200 text/javascript` |
| Secret scan | `git ls-files`, `git log --all --name-only`, `git grep -Ei '(api[_-]?key\|secret\|password\|token\|BEGIN .*PRIVATE KEY\|AKIA…\|ghp_…\|xox…)'` | Pass — no credentials in the working tree or in history |
| Directory listing suppressed (reconciliation) | `curl -o /dev/null -w '%{http_code}' :3000/assets/` and `/assets` | `404` for both — was a full HTML index of every asset |
| Real assets still served (reconciliation) | `curl` `/assets/index-DXHSlzCY.js`, `/assets/index-DeD3ddiH.css` | `200`, `Content-Type: text/javascript; charset=utf-8` / `text/css` — MIME mapping intact |
| Security headers (reconciliation) | `curl -I :3000/` | `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin` |
| Image rebuild + hardened run (reconciliation) | `docker build .`, `docker run --read-only --cap-drop=ALL` | Pass — `/health` → `200`, `/` → `200`, `/nope` → `404` |

Everything in this table reflects the repository **as committed at the end of this
audit**. The whole chain was re-run from a wiped `node_modules` after the last
change. Rows marked *(reconciliation)* are from the cross-template pass on the same
date, which closed findings 4 and 5.

## Dependency status

**npm (all devDependencies — nothing ships to the browser bundle from these):**

| Package | Before | After | Kind |
|---|---|---|---|
| `vite` | 8.0.16 | 8.2.0 | minor — applied |
| `typescript` | 6.0.3 | 7.0.2 | **major** — applied after validation (see below) |
| `postcss` (transitive, via vite) | 8.5.15 | 8.5.25 | pulled in by the vite bump; clears the high advisory |

`npm outdated` is now empty.

**Go (`server/`):** no third-party modules at all — the server is stdlib-only, so
there is no `go.sum` and no transitive dependency surface. The only version
declaration is the `go` directive, raised `1.23` → `1.26` to match the builder.

**Container base images:**

| Image | Before | After |
|---|---|---|
| build stage (web) | `node:22-alpine` | `node:24-alpine` |
| build stage (server) | `golang:1.26-alpine` | unchanged — 1.26 is current; no `golang:1.27` tag is published |
| runtime | `gcr.io/distroless/static-debian12:nonroot` | `gcr.io/distroless/static-debian13:nonroot` |

**On the TypeScript 7 major.** It was applied because the diff is a single version
line and the blast radius here is unusually small: TypeScript is not in the build
path (`vite build` strips types via rolldown and never invokes `tsc`) and no
TypeScript output reaches the shipped image — it exists for editors and
`tsc --noEmit`. It was validated three ways: `tsc --noEmit` passes, a deliberately
injected type error still fails the check (so the upgrade did not silently stop
enforcing `strict` / `noUnusedLocals`), and the full `docker build` succeeds with
the regenerated lockfile. Note that TS 7 ships per-platform native binaries as
optional dependencies, which is why `package-lock.json` grew substantially. If a
customer adds tooling that is not yet TS-7-compatible, reverting is
`"typescript": "^6"` + `npm install` and nothing else.

**Deliberately not upgraded:** nothing. There is no held-back upgrade — every
outdated package was taken.

## Findings

1. **(high) postcss path traversal — FIXED.** `postcss` 8.5.15, reached transitively
   through `vite` 8.0.16, is covered by GHSA-r28c-9q8g-f849 and GHSA-fxqj-rqcc-2cmp
   (attacker-controlled `sourceMappingURL` causes arbitrary `.map` file disclosure).
   Fixed by upgrading vite to 8.2.0, which requires `postcss ^8.5.23` and resolved
   to 8.5.25. `npm audit` is now clean. Exposure was build-time only — postcss never
   reaches the shipped image — but every app generated from this template inherited
   the vulnerable lockfile.

2. **(medium) Go toolchain drift — FIXED.** `server/go.mod` declared `go 1.23` while
   the Dockerfile builds with `golang:1.26-alpine` (this is the drift tracked as T3
   in `_planning/template-static-site-findings.md`). Assessed as *not* a live CVE:
   the go directive sets the language version, and the binary is linked against the
   1.26 stdlib regardless, which is why `govulncheck` reports clean either way. It
   was still worth fixing — the declared version misrepresented what the code is
   actually built with, and it would silently permit language constructs the
   template's stated floor does not support. Raised to `go 1.26` with a comment
   naming the Dockerfile as the source of truth. Aligning downward was rejected: the
   1.26 builder was adopted deliberately in commit `c7a987b` to clear stdlib CVEs.

3. **(medium) `.dockerignore` did not exclude local env files — FIXED.** The web
   stage runs `COPY . .`, so any `.env`, `.env.*`, or `*.local` file in a developer's
   working tree was copied into that build layer. Those exact patterns are in
   `.gitignore`, which means they are invisible to git-based secret scanning — the
   worst combination. The final image is unaffected (only `/app/dist` is copied
   forward) and builder layers are not pushed, so this was a latent path rather than
   an active leak, but it is a needless one in a template every customer inherits.
   Added `.env`, `.env.*`, `*.local`, `.DS_Store`, and `_planning` to `.dockerignore`.

4. **(low) Directory listings are served — FIXED in the reconciliation pass.**
   `http.FileServer` auto-indexes any directory without an `index.html`. Confirmed
   live against the built image: `GET /assets/` returned an HTML listing of every
   hashed asset filename. This is information disclosure of low severity (the assets
   are public anyway), but it is not behaviour a static site should have by default —
   and it upgrades every unreferenced draft file a customer drops in `public/` from
   "unlinked" to "discoverable by browsing".
   Originally deferred here on the grounds that it changes runtime behaviour for every
   generated app and deserved its own reviewed diff. The cross-template pass is that
   diff: `template-3d-web-app` found the same issue and fixed it, and shipping two
   near-identical static servers with different behaviour is worse than either choice
   on its own. **Action taken:** ported the `template-3d-web-app` fix — an
   `indexOnlyDirs` wrapper around the `http.FileSystem` that returns `os.ErrNotExist`
   for a directory with no `index.html`, so `http.FileServer` 404s instead of listing.
   Verified live: `/assets/` and `/assets` both `404`, real hashed assets still `200`
   with correct MIME types.

5. **(low) No security response headers — FIXED in the reconciliation pass.**
   Responses carried no `X-Content-Type-Options`, `Referrer-Policy`, `X-Frame-Options`,
   or `Content-Security-Policy`. **Action taken:** added a `securityHeaders` middleware
   setting `X-Content-Type-Options: nosniff` and
   `Referrer-Policy: strict-origin-when-cross-origin` on every response, matching
   `template-3d-web-app` exactly.
   `Content-Security-Policy` and `X-Frame-Options` are still deliberately **not** set,
   also matching 3d-web-app: a useful CSP depends on where the customer's app loads
   content from, and framing a generated site in another page is a legitimate use of
   this template. Both are documented in the source as decisions for the app to make.

6. **(low) Base-image currency — FIXED.** `node:22-alpine` is in maintenance LTS and
   `distroless/static-debian12` is one Debian release behind. Neither carried a known
   exploited vulnerability for this image, but a template should start customers on
   current bases, not on ones nearing end of support. Moved to `node:24-alpine`
   (active LTS, build stage only — no Node reaches the runtime image) and
   `distroless/static-debian13:nonroot`. `golang:1.26-alpine` was checked and is
   already the newest published major.

7. **(low) Non-root was implicit — FIXED.** The runtime stage had no `USER`
   directive and relied entirely on the `:nonroot` tag's baked-in uid 65532. Correct,
   but fragile: a customer editing the `FROM` line to a non-`nonroot` tag would
   silently start running as root with no other visible change. Added an explicit
   `USER nonroot:nonroot`. Verified that the platform's own `runAsUser: 10001`
   override still serves correctly (the binary and `/www` are world-readable), and
   that the container runs happily with `--read-only` and `--cap-drop=ALL`.

8. **(low) Typecheck failed out of the box — FIXED.** `npx tsc --noEmit` failed with
   `TS2882: Cannot find module or type declarations for side-effect import of
   './style.css'` because `vite/client` types were never referenced. Every app
   generated from this template showed a type error on line 1 of its first source
   file the moment a customer opened it in an editor. Added
   `"types": ["vite/client"]` to `tsconfig.json`. This also unblocks tracked item T4.

9. **(low) The template ships no tests — PARTIALLY FIXED in the reconciliation pass.**
   There was no `npm test` script and no Go test files, so `npm test` errored and
   `go test ./...` reported "no test files". A template with zero tests establishes a
   zero-test baseline for every app generated from it. The **Go side is now covered**:
   `server/main_test.go` came in with the findings 4/5 fix and asserts the health
   contract, asset serving, listing suppression and the security headers. The
   **frontend side is unchanged** — still no `npm test` script, because choosing a
   frontend test runner is a product decision, not a maintenance one. See Open items.

10. **(low, accepted — not fixed by design) No `WriteTimeout` on the static server.**
    The reconciliation pass added `ReadHeaderTimeout` (10s), `ReadTimeout` (30s) and
    `IdleTimeout` (120s) — which is what closes the Slowloris hole — but deliberately
    no `WriteTimeout`. A static site serves potentially large assets, and a client on
    a slow mobile link is a legitimate long write that a `WriteTimeout` would truncate
    mid-download, producing corrupt assets that look like a CDN fault. This is an
    intentional split from the API templates (`template-container`,
    `template-go-web-service`), which *do* set `WriteTimeout` because a JSON API can
    bound its response writes; a file server cannot. Recorded so a future audit does
    not "harmonise" the inconsistency in the wrong direction. `template-3d-web-app`
    makes the same call for the same reason.

11. **(none) No secrets, in the tree or in history.** All 15 tracked files were
    scanned along with every path that has ever existed in this repository. The only
    file ever deleted is `.github/workflows/ci.yml`, removed deliberately (the
    platform builds in-cluster and never reads GitHub Actions). No keys, tokens, or
    credentials found.

## Changes made in this audit
- `package.json`: `vite` `^8.0.16` → `^8.2.0`; `typescript` `^6.0.3` → `^7.0.2`.
- `package-lock.json`: regenerated (`postcss` 8.5.15 → 8.5.25, clearing the high advisory).
- `server/go.mod`: `go 1.23` → `go 1.26`, with a comment naming the Dockerfile builder as the source of truth.
- `Dockerfile`: `node:22-alpine` → `node:24-alpine`; `distroless/static-debian12:nonroot` → `distroless/static-debian13:nonroot`; added an explicit `USER nonroot:nonroot`.
- `.dockerignore`: exclude `.env`, `.env.*`, `*.local`, `.DS_Store`, `_planning`.
- `tsconfig.json`: added `"types": ["vite/client"]` so `tsc --noEmit` passes.
- Added this file.

Added in the **cross-template reconciliation pass** (same date), closing findings 4
and 5 by porting `template-3d-web-app`'s static-server hardening so the two
near-identical Go static servers no longer ship different security postures:

- `server/main.go`: restructured into a testable `newHandler(http.FileSystem)`; added
  an `indexOnlyDirs` wrapper that 404s directories with no `index.html` (finding 4);
  added a `securityHeaders` middleware setting `X-Content-Type-Options: nosniff` and
  `Referrer-Policy: strict-origin-when-cross-origin` (finding 5); replaced the bare
  `http.ListenAndServe` with an `http.Server` carrying `ReadHeaderTimeout` 10s,
  `ReadTimeout` 30s and `IdleTimeout` 120s — and deliberately **no** `WriteTimeout`
  (finding 10). CSP and `X-Frame-Options` left to the app, as in 3d-web-app.
- `server/main_test.go`: **new** — covers `/health`, index and asset serving, the
  directory-listing 404, and the two security headers. Closes open item 4; the Go
  module now has tests rather than `[no test files]`.

`onklave.yaml` was not touched. No page markup, styles, or README content was changed.
The Dockerfile copies `server/go.mod` and `server/main.go` explicitly, so the new test
file is correctly excluded from the image.

## Open items
1. **Confirm the TypeScript 7 major.** Validated and safe as the template stands
   today, but it is the one judgement call here and a human may prefer a template to
   sit on the previous major. Revert = `"typescript": "^6"` + `npm install`.
2. ~~**Decide on findings 4 and 5** (directory listings, security headers).~~
   **Closed** by the reconciliation pass — both fixed in `server/main.go` with tests,
   matching `template-3d-web-app`. Nothing further needed. The one behaviour change to
   communicate to existing generated apps: a directory URL with no `index.html` now
   returns 404 instead of an auto-generated listing.
3. **Tracked item T4** (`_planning/template-static-site-findings.md`): add a
   `"typecheck": "tsc --noEmit"` script and decide whether to fold it into `build`.
   This audit fixed the reason it could not be done — `tsc --noEmit` now passes.
4. ~~**Consider one Go test** for the `/health` handler.~~ **Closed** by the
   reconciliation pass — `server/main_test.go` now covers the health contract, asset
   serving, listing suppression and security headers. (Finding 9's point still stands
   for the *frontend*: there is still no `npm test`.)
5. **Automate this.** A quarterly manual pass is how the postcss advisory sat here in
   the first place. Either enable Renovate/Dependabot on the template repos or put a
   scheduled Onklave agent run on them, so the next high-severity advisory raises a
   diff instead of waiting for a review date.
