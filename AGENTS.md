# Jury agent guide

Jury is a mobile-first hackathon judging system.  Keep changes small, consistent with the
surrounding feature, and mindful that the admin and judge flows are used during live events.

## Repository map

- `client/` — React 18, TypeScript, Vite, Tailwind CSS, and Zustand frontend.
- `server/` — Go 1.23 Gin API and MongoDB application.  Its module is separate from the
  repository root.
- `tests/` — separate Go module containing API integration tests.
- `docs/` — Docusaurus documentation; update it when user-facing behavior, configuration,
  API contracts, or development workflows change.
- Compose files at the root — the supported full-stack development and test environments.

Before editing, read the nearest analogous feature and its tests.  The source of truth for API
registration is `server/router/init.go`; frontend calls are centralized in `client/src/api.ts`.

## Local workflow

Use the smallest validation that covers the change, then run broader checks for cross-stack work.

```bash
# Frontend (from client/)
yarn install
yarn lint
yarn build

# Format Go sources (from repository root; both are separate Go modules)
gofmt -w $(find server tests -name '*.go' -print)

# Compile and run Go package tests (run separately because they are separate modules)
(cd server && go test ./...)
(cd tests && go test ./...)

# Full integration suite (requires Docker and the expected environment variables)
./scripts/test-runner.sh
# equivalently: docker compose -f docker-compose.test.yml up
```

The normal development environment is Docker Compose:

```bash
docker compose -f docker-compose.dev.yml up
# or, with a local MongoDB container:
docker compose -f docker-compose-mongo.dev.yml up
```

Copy the documented `.env.template` files rather than committing `.env` files or credentials.
Do not reset, seed, or otherwise mutate a shared/production database while validating a change.

## Frontend conventions

- Use TypeScript with strict types.  Place shared request/response shapes in `client/src/types.d.ts`;
  keep feature-specific props next to their component.
- Pages live in `client/src/pages/`.  Reusable components go in `client/src/components/`; put
  page-specific components in a matching subdirectory such as `components/admin/` or
  `components/judge/`.
- Define a named props interface with concise comments for each prop.  Use `React.ReactNode` for
  `children` and an optional `className?: string` for Tailwind overrides.
- Prefer existing primitives (`Button`, `Card`, inputs, popups, `Container`) and Tailwind classes
  over a new dependency or a component library.  Combine overridable class strings with
  `twMerge` where an existing component does so.
- Preserve the mobile-first layout; responsive Tailwind prefixes add larger-screen behavior.
- Call the backend only through `getRequest`, `postRequest`, `putRequest`, or `deleteRequest` in
  `client/src/api.ts`.  Supply the appropriate `admin` or `judge` auth mode, type the response,
  and handle non-200 responses with `errorAlert` before reading data.
- Put shared client state and fetch actions in the appropriate Zustand store in `client/src/store.tsx`.
  Do not duplicate stale copies of shared server state in individual pages.
- Formatting is Prettier: single quotes, 4-space indentation, and 100-character print width.

## Backend conventions

- Keep HTTP handlers in `server/router/`, grouped by the resource they manage.  Register every
  route in `server/router/init.go` using the correct default, judge-authenticated, or
  admin-authenticated group.
- A handler accepts only `ctx *gin.Context`, reads shared services with `GetState(ctx)`, validates
  request input early, and returns JSON errors as `gin.H{"error": ...}` with a fitting HTTP status.
  Successful mutation endpoints normally return `gin.H{"ok": 1}`.
- Keep persistence in `server/database/`, domain data in `server/models/`, judging algorithms in
  `server/judging/`, and specialized helpers in `server/funcs/` or `server/util/`.  Do not embed
  MongoDB queries or scoring logic directly in a route handler.
- Use the context-authenticated judge only on judge routes (`ctx.MustGet("judge")`); never trust a
  judge identity supplied by the client body.
- Preserve transactions around multi-write operations and reload dependent comparison state when
  project data changes, following the adjacent route implementation.
- Log meaningful admin/judge mutations through `state.Logger` without logging passwords, tokens,
  connection strings, or other secrets.
- Run `gofmt` on touched Go files.  Follow Go naming and package conventions rather than importing
  a new formatting or lint tool without project agreement.

## Tests, documentation, and change hygiene

- Add or update an integration test in `tests/tests/` for an API behavior change, including an
  error case where relevant.  Use the existing request/assertion helpers in `tests/util/` rather
  than raw HTTP or database calls.
- For frontend-only changes, run lint and production build when dependencies are available.  For
  backend or contract changes, run the Docker integration suite when Docker is available.
- State explicitly in the handoff which checks ran and which could not run, with the reason.
- Update the relevant page in `docs/docs/` for changed configuration, routes, architecture, or
  user-facing workflows.  Keep docs concise and describe the shipped behavior, not an intention.
- Avoid drive-by formatting, generated output, lockfile churn, unrelated refactors, and dependency
  additions.  This project deliberately minimizes third-party software and production image size.
- Do not overwrite existing user changes.  Inspect `git status` first, limit edits to the requested
  scope, and do not commit unless asked.

## Definition of done

A change is ready when it preserves the appropriate auth and API contract, follows the relevant
layer boundaries, includes proportional tests and documentation, is formatted, and reports the
validation result honestly.
