# Repository Guidelines

## Project Structure & Module Organization
- `core/` multi-tenant service built with Go Zero; API definitions under `core/api`, ent schemas in `core/rpc/ent`, automation scripts in `core/scripts`.
- `nb-agent/` agent server + CLI Go module; main entrypoints `cmd/nb` and `cmd/cli`, docs in `nb-agent/docs`.
- `common/` shared Go packages reused by services; pay attention to `common/tenant` and `common/middleware` for cross-cutting hooks.
- `ui/` pnpm-managed Vue 3 monorepo; apps under `ui/apps`, shared assets in `ui/packages`, deploy scripts in `ui/scripts`.
- Global configs live in `configs/`, architecture references under `docs/`; Go modules are coordinated by the root `go.work`.

## Build, Test, and Development Commands
- `cd core && make test` runs API and RPC unit tests (`go test -v --cover`); `make gen-rpc` regenerates protobuf scaffolding after schema updates.
- `cd nb-agent && make agent` builds the server binary; use `make cli` for the CLI and `make test` for Go unit + integration tests.
- `cd ui && pnpm install` bootstraps Node deps (Node >=20, pnpm >=9); `pnpm dev` launches the Turbo dev server, `pnpm build` produces production bundles.
- `cd common && go test ./...` validates shared libraries before consuming them downstream.
- Run `make fmt`/`make lint` (Go) and `pnpm lint`/`pnpm format` (UI) to keep tooling green prior to reviews.

## Coding Style & Naming Conventions
- Go code must stay `gofmt` clean; run `make fmt` or `go fmt ./...` and favor short, lowercase package names (for example `tenant`, `hooks`).
- Follow `CLAUDE.md` for ent schema updates: modify `rpc/ent/schema`, regenerate via `make gen-ent`, then `make gen-rpc` before wiring new logic.
- Front-end modules inherit configs from `@vben/eslint-config` and `@vben/prettier-config`; keep components in PascalCase `.vue` files and composables in `useXyz` camelCase.
- Configuration files (`configs/*.yaml`, `config/*.yml`) use snake_case keys to align with current loaders.

## Testing Guidelines
- Minimum bar: `make test` in each Go module plus targeted tenant isolation tests (see `CLAUDE.md` §4) whenever touching data access.
- UI unit tests run via `pnpm test:unit`; end-to-end coverage uses Playwright with `pnpm test:e2e`.
- Name Go tests `Something_test.go` colocated with sources; prefer table-driven cases and assert tenant/data-permission boundaries explicitly.

## Commit & Pull Request Guidelines
- Commits follow Conventional Commits (`feat:`, `fix:`, `chore:` …); use `pnpm commit` or `czg` to stay consistent and include module scopes (`core`, `nb-agent`, `ui`).
- Reference related issues in the body and keep changes isolated; scripts embed build metadata, so avoid committing generated binaries.
- PRs should list affected services, test evidence (`make test`, `pnpm test:unit` output), and screenshots for UI updates; link architecture docs when changing tenant or permission flows.

## Security & Tenant Controls
- Treat tenant and data-permission hooks as critical: never bypass ent hooks or issue raw SQL inside `core`/`nb-agent`.
- Register middleware via `common/middleware/integration` and ensure APIs include `TenantCheck` and `DataPerm` where applicable before merging.

## Other
- 任何时候使用中文回答问题
