# Contributing

Open an issue or a pull request on
[GitHub](https://github.com/Haibread/ai-registry). The short version: branch
from `main`, change the OpenAPI spec before the code when you touch the API,
keep `pre-commit run --all-files` and the test suites green, and open a PR.

How the system fits together is in [ARCHITECTURE.md](ARCHITECTURE.md); running
it is in [README.md](README.md#getting-started).

## Development setup

### Tooling

| Tool | Version | Used for |
| --- | --- | --- |
| Go | as in [server/go.mod](server/go.mod) | server build and tests |
| Node.js + npm | 26 | SPA build, tests, lint |
| Docker + Compose plugin | recent | integration tests (testcontainers), local stack |
| pre-commit | ≥ 3.2 | commit gate |
| golangci-lint | v2.14.0 | `golangci-lint` hook |
| helm | v4.3.0 | `helm-lint` hook, chart rendering |
| helm-docs | v1.14.2 | `helm-docs` hook |
| hadolint | v2.12.0 | `hadolint` hook |
| actionlint | v1.7.12 | `actionlint` hook |
| kubeconform | v0.6.7 | chart schema validation ([deploy/helm/validate.sh](deploy/helm/validate.sh)) |
| trivy | recent | reproducing the CI image and config scans |

The versions are the ones CI installs. Go and Node come from their upstream
installers: [go.dev/doc/install](https://go.dev/doc/install) and
[nodejs.org/en/download](https://nodejs.org/en/download). The rest:

```bash
pipx install pre-commit
```

```bash
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b "$(go env GOPATH)/bin" v2.14.0
```

```bash
go install github.com/norwoodj/helm-docs/cmd/helm-docs@v1.14.2
```

```bash
go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
```

```bash
go install github.com/yannh/kubeconform/cmd/kubeconform@v0.6.7
```

```bash
curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-4 | DESIRED_VERSION=v4.3.0 bash
```

hadolint:

**macOS**

```bash
brew install hadolint
```

**Linux** — download the `hadolint-Linux-x86_64` (or `-arm64`) binary of
[v2.12.0](https://github.com/hadolint/hadolint/releases/tag/v2.12.0) onto your
`PATH`.

### Bootstrap

From a fresh clone:

```bash
cd web && npm ci
```

```bash
pre-commit install
```

This installs both the `pre-commit` and the `commit-msg` hook; in a clone that
had hooks installed before `commit-msg` was added, run it once more.

The setup is good when this passes:

```bash
pre-commit run --all-files
```

## Running the tests

Three layers, each with different needs.

### Server — unit and integration

```bash
cd server && go test -race ./...
```

Table-driven unit tests cover `domain`, `auth`, `config` and the middleware.
Repository and handler tests start a throwaway PostgreSQL with
[testcontainers](https://golang.testcontainers.org/), so **Docker must be
running**. A single package or test:

```bash
cd server && go test -run TestRefreshToken_ReuseRevokesLineage ./internal/store/
```

CI fails when total Go coverage drops below **70 %**. To see where you stand:

```bash
cd server && go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out | tail -1
```

The contract suites are part of `go test ./...`; they assert that the OpenAPI
operations and the chi routes match one-to-one, that every write route is
guarded, and that emitted Agent Cards conform to the pinned A2A schema. Adding
a route without its spec entry, or the reverse, fails them.

### Web — unit and component

```bash
cd web && npm run test:coverage
```

Vitest with Testing Library, no services needed; `npm test` runs the same
suite without coverage, `npm run test:watch` while iterating. CI runs
`npm run test:coverage`, which fails when total coverage drops below the floor
in `coverage.thresholds` of [`web/vitest.config.ts`](web/vitest.config.ts).
Public and admin pages are both measured.

### Web — nginx routing

```bash
test/nginx/routing.sh
```

Runs [web/nginx.conf](web/nginx.conf) in the web image's nginx base in front
of a stub backend (Docker required, no build). It checks that `/metrics`
answers 404 instead of being proxied to the server, that the SPA gets its
security headers, and that proxied responses keep the backend's headers
without the SPA's stacked on top.

### End to end

Playwright drives the SPA against a real server and Keycloak. Start the local
stack, then run the suite against it (default base URL
`http://localhost:3000`, override with `E2E_BASE_URL`):

```bash
docker compose --profile dev up -d --build
```

```bash
cd web && npx playwright install chromium
```

```bash
cd web && npm run test:e2e
```

The test users are the dev realm's
([deploy/keycloak-realm-dev.json](deploy/keycloak-realm-dev.json)); override
them with `E2E_<ROLE>_EMAIL` / `E2E_<ROLE>_PASSWORD`. A k6 smoke test lives in
[test/load/](test/load/README.md).

### What a change needs

New behaviour comes with tests at the right layer: unit tests for logic, an
integration test for a new handler or query, a Playwright spec for a new admin
flow. A bug fix comes with the test that would have caught it.

### Changing the API

1. Edit [server/api/openapi.yaml](server/api/openapi.yaml).
2. Regenerate the TypeScript client — CI fails if the committed file differs:

   ```bash
   cd web && npm run generate
   ```

3. Implement the handler and route; the contract suite tells you if they
   disagree with the spec.

A schema change is a new file pair in [server/migrations/](server/migrations/)
(`NNNNNN_name.up.sql` / `.down.sql`). Migrations are forward-only: never edit
one that has been merged.

## Pre-commit hooks

Hooks must pass before you push; CI runs the same set, so skipping them locally
only moves the failure somewhere slower. `--no-verify` and `SKIP=` are not a
fix — correct the code, or change the rule in
[.pre-commit-config.yaml](.pre-commit-config.yaml) in the same PR and say why.

```bash
pre-commit run --all-files
```

```bash
pre-commit run golangci-lint --all-files
```

| Hook | Checks | Fix |
| --- | --- | --- |
| `trailing-whitespace`, `end-of-file-fixer` | whitespace | auto-fixed, re-stage |
| `check-yaml`, `check-merge-conflict`, `check-added-large-files`, `detect-private-key` | YAML syntax, conflict markers, large files, private keys | by hand |
| `gitleaks` | secrets in the diff | remove the secret, rotate it |
| `yamllint` | YAML style: block style only, no leading `---` ([.yamllint.yaml](.yamllint.yaml)); Helm templates excluded | by hand |
| `shellcheck` | shell scripts | by hand |
| `sqlfluff-lint` | SQL style, PostgreSQL dialect ([.sqlfluff](.sqlfluff)); migrations `000001`–`000023` excluded, since merged migrations are never edited | by hand, or `sqlfluff fix <file>` |
| `gofmt` | Go formatting | auto-fixed, re-stage |
| `go-vet`, `golangci-lint` | Go correctness and lint ([server/.golangci.yml](server/.golangci.yml)) | by hand |
| `web-eslint`, `web-tsc` | SPA lint and type-check (needs `npm ci` in `web/`) | by hand |
| `helm-lint` | chart validity | by hand |
| `helm-docs` | chart README matches `values.yaml` | auto-regenerated, re-stage |
| `hadolint` | Dockerfiles | by hand |
| `actionlint` | GitHub workflows | by hand |
| `no-co-authors` (`commit-msg` stage) | no `Co-Authored-By:` or `Generated with` line in the commit message | reword the commit |

## Continuous integration

Workflows live in [.github/workflows/](.github/workflows/). The only secret
they use is the built-in `GITHUB_TOKEN`, so fork PRs run Lint, Quality and the
Docker build like any other.

| Workflow | Triggers on | What it does | Reproduce locally |
| --- | --- | --- | --- |
| [Lint](.github/workflows/lint.yml) | PR to `main`, manual | `pre-commit run --all-files` | `pre-commit run --all-files` |
| [Quality](.github/workflows/quality.yml) | PR to `main`, manual | Go build, `go test -race` with the 70 % floor, contract suites; web `npm run generate` drift check, `npm run build`, `npm run test:coverage` with its floor; nginx routing checks; `helm lint`, `helm template` over several value sets + kubeconform ([deploy/helm/validate.sh](deploy/helm/validate.sh)); e2e (Postgres, Keycloak, server, Vite, k6 smoke, Playwright) | the commands in [Running the tests](#running-the-tests) |
| [Docker](.github/workflows/docker.yml) | PR to `main`, manual: build, no push. Push to `main` or a `v*.*.*` tag: build and push to GHCR | multi-arch (`amd64`, `arm64`) server and web images. Without a push, an `amd64` copy is loaded and scanned with `trivy image`; after a push, the pushed digest is scanned. Fails on a fixable `HIGH`/`CRITICAL` CVE; all findings go to code scanning | [Scanning locally](#scanning-locally) |
| [Security](.github/workflows/security.yml) | PR to `main`, push to `main`, manual | `trivy config` over the Dockerfiles and the Helm chart; fails on a `HIGH`/`CRITICAL` misconfiguration, all findings go to code scanning | `trivy config --severity HIGH,CRITICAL --exit-code 1 .` |
| [Helm publish](.github/workflows/helm-publish.yml) | push to `main`, `chart-*` tag | runs [deploy/helm/validate.sh](deploy/helm/validate.sh), then packages and pushes the chart to GHCR as OCI; nothing is published if validation fails | `deploy/helm/validate.sh && helm package deploy/helm/ai-registry` |
| [Release](.github/workflows/release.yml) | a successful Docker run for a `vX.Y.Z` tag (not a pre-release) | GitHub Release with notes generated from PR labels ([release.yml](.github/release.yml)) | — |

A PR is ready to merge when Lint, Quality, Docker and Security are green.
Code-scanning uploads are skipped on fork PRs, whose token cannot write them;
the gates still run.

### Scanning locally

The image gate, shown for the server (for the web image, build with
`-f web/Dockerfile .` and tag it `ai-registry-web:scan`):

```bash
docker buildx build --platform linux/amd64 --load -t ai-registry-server:scan server
```

```bash
trivy image --ignore-unfixed --severity HIGH,CRITICAL --exit-code 1 ai-registry-server:scan
```

A finding is fixed at the source — a base-image or dependency bump, or the
misconfiguration itself — not added to a `.trivyignore`.

`trivy config` reads [`trivy.yaml`](trivy.yaml) from the repository root, which
loads the check parameters under [`.trivy/`](.trivy/) — the registries
`KSV-0125` trusts, for instance. An image published to a new registry is added
there.

### Releases

Application and chart are versioned independently:

- a **`v1.2.3`** tag publishes the server and web images at that version and
  cuts the GitHub Release, its notes generated from the labels of the PRs
  merged since the previous stable release;
- a **`v1.2.3-rc1`** pre-release tag publishes the images only, for testing:
  no GitHub Release, and its PRs appear in the notes of `v1.2.3`;
- a **`chart-1.2.3`** tag publishes the chart at that version, with the
  `appVersion` committed in
  [Chart.yaml](deploy/helm/ai-registry/Chart.yaml).

[scripts/release.sh](scripts/release.sh) is the only supported way to cut
either. From an up-to-date, clean `main`, it writes the version into the
repository, commits, tags, and asks before pushing — the push is what
publishes:

```bash
scripts/release.sh 1.2.3                          # application: web/package.json, tag v1.2.3
scripts/release.sh --chart 0.5.0                  # chart: Chart.yaml + README, tag chart-0.5.0
scripts/release.sh --chart --app-version 1.2.2 0.5.1
scripts/release.sh --dry-run 1.2.3                # show the plan, change nothing
```

A chart release deploys the latest `v*` tag unless
`--app-version` names another. The Docker and Helm publish workflows refuse a
tag that disagrees with the committed version, so a hand-made tag fails
instead of publishing.

### Dependency updates

[Dependabot](.github/dependabot.yml) owns Go modules, npm, the Dockerfiles,
the Compose file, GitHub Actions and the pre-commit hooks;
[Renovate](renovate.json) only bumps the tool and image versions pinned inside
workflow `run:` steps and service containers, which Dependabot cannot see. Both
open PRs on Mondays; none is merged automatically.

## Submitting a change

- Branch from `main` as `feat/<topic>`, `fix/<topic>`, `docs/<topic>` or
  `chore/<topic>`.
- Commits follow [Conventional Commits](https://www.conventionalcommits.org/)
  (`feat:`, `fix:`, `docs:`, `chore:`, `test:`), as in `git log --oneline`.
- Update [ARCHITECTURE.md](ARCHITECTURE.md), the README or
  [deploy/config.example.yaml](deploy/config.example.yaml) in the same PR when
  the change affects them. A new setting goes in
  [server/internal/config/config.go](server/internal/config/config.go), the
  example config and [deploy/.env.example](deploy/.env.example) together.
- Label the PR with the release-notes category it belongs to: `feature`,
  `fix`, `security`, `performance`, `deprecation`, `documentation`,
  `dependencies` or `chore`, plus `breaking` when it breaks something, or
  `ignore-for-release` to keep it out of the notes. The categories are in
  [.github/release.yml](.github/release.yml).
