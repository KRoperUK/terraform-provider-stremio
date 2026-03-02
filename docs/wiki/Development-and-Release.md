# Development and Release

## Local Quality Checks

```bash
go test ./...
terraform fmt -recursive
pre-commit run --all-files
```

## CI

- Workflow: `.github/workflows/ci.yaml`
- Runs pre-commit only against files changed vs default branch.

## Release Automation

- Workflow: `.github/workflows/release-please.yml`
- Config: `release-please-config.json`
- Manifest: `.release-please-manifest.json`

Release Please creates/updates a release PR from conventional commits. Merging the release PR creates a GitHub release and uploads provider build artifacts.

## Commit Conventions

Use conventional commits:

- `feat: ...`
- `fix: ...`
- `docs: ...`
- `chore: ...`
