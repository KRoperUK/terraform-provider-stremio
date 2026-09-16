<!--
PR titles are used as the squash-merge commit message and are read by
release-please, so the title MUST be a Conventional Commit, e.g.:
  feat: add stremio_library data source
  fix(account): handle empty transport_urls on import
-->

## Summary

Describe what this PR changes and why.

## Type of Change

- [ ] feat
- [ ] fix
- [ ] docs
- [ ] refactor
- [ ] test
- [ ] ci / build
- [ ] chore

## Related Issues

Closes #

## Terraform Provider Impact

- Resources affected:
- Data sources affected:
- Breaking changes:

## Validation

- [ ] `go test -race ./...`
- [ ] `go vet ./...`
- [ ] `golangci-lint run ./...`
- [ ] `terraform fmt -recursive examples/`
- [ ] `pre-commit run --all-files`
- [ ] Docs regenerated (if any schema/example changed)
- [ ] Manual validation done (describe below)

### Manual Validation Notes

<!-- Include plan/apply output summary and tested examples -->

## Release Notes

<!-- Write one concise bullet suitable for changelog/release notes -->

-

## Checklist

- [ ] My commits are signed and use Conventional Commit messages
- [ ] I updated docs/README/examples if behavior or schema changed
- [ ] I did not commit secrets (`.env`, `*.tfvars`, `*.tfstate`)
