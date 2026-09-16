# Contributing

Thanks for your interest in improving the Stremio Terraform provider! This guide
covers how to set up your environment, make changes, and get them merged.

By participating you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md).

## Prerequisites

- [Go 1.25+](https://go.dev/dl/)
- [Terraform 1.6+](https://developer.hashicorp.com/terraform/install)
- [pre-commit](https://pre-commit.com/) (`pip install pre-commit`)
- A GPG (or SSH) signing key registered with GitHub — **all commits must be signed.**

## Getting started

```bash
git clone https://github.com/KRoperUK/terraform-provider-stremio.git
cd terraform-provider-stremio
go mod download
pre-commit install        # run the hooks on every commit
make build                # compile the provider
```

To try the provider locally against the examples, copy `.env.example` to `.env`,
fill in `STREMIO_EMAIL` / `STREMIO_PASSWORD`, and use the helper scripts (they wire
up Terraform `dev_overrides` for you):

```bash
./scripts/plan.sh -no-color
./scripts/apply.sh -auto-approve
```

> **Never commit secrets.** `.env`, `*.tfvars`, and `*.tfstate` are ignored on
> purpose — keep credentials out of the repository and rotate anything exposed.

## Project layout

| Path | What lives there |
| --- | --- |
| `main.go` | Provider entrypoint |
| `internal/provider/` | Provider, resources, data sources, and the Stremio API client |
| `internal/provider/client.go` | RPC client (`POST /api/<method>`); unit-tested in `client_test.go` |
| `docs/` | Registry docs — **generated**, do not edit by hand (except `docs/wiki/`) |
| `examples/` | Usage examples surfaced in the generated docs |
| `scripts/` | Local dev helpers |

The provider exposes the `stremio_account` resource (add-on collections are managed
via its `transport_urls` set) and the `stremio_installed_addons`,
`stremio_watch_history`, and `stremio_continue_watching` data sources. There is no
separate `stremio_addon_collection` resource. See [CLAUDE.md](CLAUDE.md) for a deeper
tour aimed at both humans and AI agents.

## Making changes

1. **Branch** off `main`.
2. **Write code** that matches the existing terraform-plugin-framework patterns
   (typed `tfsdk` models, `types.*`, diagnostics for errors). Keep schema
   `Description` / `MarkdownDescription` strings accurate — the registry docs are
   generated from them.
3. **Add or update tests** in `internal/provider`. Mock HTTP with
   `net/http/httptest`; never call the live Stremio API in tests.
4. **Regenerate docs** if you touched any schema or example:

   ```bash
   go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@v0.25.0 generate --provider-name stremio
   ```

   Commit the regenerated files — CI fails if `docs/` (excluding `docs/wiki/`) is
   out of date. Keep this pinned version in sync with `.github/workflows/ci.yaml`
   and `.pre-commit-config.yaml`.
5. **Update the README / examples** so they match the actual schema. Drift between
   the docs and the code is treated as a bug.

## Before you push

Run the same checks CI does:

```bash
gofmt -l .                 # should print nothing
go vet ./...
golangci-lint run ./...
go test -race ./...
terraform fmt -recursive examples/
pre-commit run --all-files
```

## Commits

- Use [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`,
  `docs:`, `test:`, `ci:`, `build:`, `chore:`, `refactor:`. `feat:` and `fix:` drive
  releases (via release-please); the others do not.
- **Sign every commit** (`git commit -S`, or set `commit.gpgsign = true` globally). A
  repository ruleset rejects unsigned commits on all branches.
- Keep commits focused; write a clear body explaining the *why*.

## Pull requests

- Fill out the [PR template](.github/pull_request_template.md).
- All status checks (`pre-commit`, `go`, `terraform-docs`) must pass before merge.
- PRs are squash-merged; the **PR title** must be a Conventional Commit message,
  because it becomes the commit on `main` that release-please reads.

## Releases (maintainers)

Releases are automated by [release-please](https://github.com/googleapis/release-please):

1. `feat:` / `fix:` commits on `main` cause release-please to open a
   `chore(main): release X.Y.Z` PR with the changelog and version bumps.
2. Merging that PR creates the `vX.Y.Z` tag.
3. The tag triggers GoReleaser, which publishes GPG-signed, registry-compatible
   artifacts. The Terraform Registry picks them up automatically.

To force a release without a `feat:`/`fix:` change (e.g. a dependency rebuild), open
a PR titled `fix(deps): …`, or add a `Release-As: X.Y.Z` footer to a commit.

## Reporting bugs and requesting features

Use the [issue templates](https://github.com/KRoperUK/terraform-provider-stremio/issues/new/choose).
For security vulnerabilities, follow [SECURITY.md](SECURITY.md) instead of opening a
public issue.
