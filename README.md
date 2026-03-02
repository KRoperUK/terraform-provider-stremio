# terraform-provider-stremio

Terraform provider for Stremio account workflows.

## Status

Early release / developer preview.

## Features

- Create a Stremio account via email/password (`stremio_account`)
- Import an existing account via `email:password`
- Read installed add-ons for the authenticated account (`stremio_installed_addons`)

## Requirements

- Go 1.22+
- Terraform 1.6+

## Build

```bash
go mod tidy
go build -o terraform-provider-stremio
```

## Setup

```bash
cp .env.example .env
# fill in STREMIO_EMAIL / STREMIO_PASSWORD
```

## Local Dev Commands

With `.env` containing `STREMIO_EMAIL` and `STREMIO_PASSWORD`:

```bash
./scripts/plan.sh -no-color
./scripts/apply.sh -auto-approve

# multi-account example
./scripts/plan-multi.sh -no-color
./scripts/apply-multi.sh -auto-approve
```

These scripts automatically:
- build the local provider binary
- configure Terraform `dev_overrides` for `kroperuk/stremio`
- pass credentials to Terraform via `TF_VAR_stremio_email` and `TF_VAR_stremio_password`

## Provider Configuration

```hcl
provider "stremio" {
  base_url = "https://api.strem.io"
}
```

You can also configure provider-level authentication:

```hcl
provider "stremio" {
  email    = var.stremio_email
  password = var.stremio_password
}
```

## Resource: stremio_account

```hcl
resource "stremio_account" "user" {
  email    = var.stremio_email
  password = var.stremio_password
}
```

### Import existing account

```bash
terraform import stremio_account.user 'user@example.com:super-secret-password'
```

## Data Source: stremio_installed_addons

```hcl
data "stremio_installed_addons" "current" {}

output "installed_addons" {
  value = data.stremio_installed_addons.current.addons
}
```

## Resource: stremio_addon_collection

Manage the full add-on collection as desired state.

```hcl
resource "stremio_addon_collection" "main" {
  transport_urls = [
    "https://v3-cinemeta.strem.io/manifest.json",
    "https://opensubtitles-v3.strem.io/manifest.json",
  ]
}
```

For multi-account management, set per-resource credentials:

```hcl
resource "stremio_addon_collection" "account" {
  for_each = var.accounts

  email    = each.value.email
  password = each.value.password

  transport_urls = var.shared_transport_urls
}
```

- Add an add-on: append a URL in `transport_urls` and run `terraform apply`.
- Remove an add-on: delete its URL from `transport_urls` and run `terraform apply`.
- The collection is authoritative: Terraform updates Stremio to match exactly this set.

## Multi-Account Example

See [examples/multi-account/main.tf](examples/multi-account/main.tf) and [examples/multi-account/variables.tf](examples/multi-account/variables.tf).

Pass `accounts` as a map of credentials and Terraform applies the same addon set to every account.

If accounts already exist, keep `create_accounts = false`.

## Notes

- The provider uses the same RPC style as `stremio-api-client`:
  - `POST /api/register`
  - `POST /api/login`
  - `POST /api/addonCollectionGet` (with `authKey` in request body)
  - `POST /api/addonCollectionSet` (with `authKey` in request body)
- If your deployment uses a different API host, set `base_url`.
- For third-party/private add-ons, ensure URLs are valid and reachable from the Stremio clients that will use those accounts.

## Releases

This repository uses Release Please via GitHub Actions:

- Workflow: [.github/workflows/release-please.yml](.github/workflows/release-please.yml)
- Config: [release-please-config.json](release-please-config.json)
- Manifest: [.release-please-manifest.json](.release-please-manifest.json)

How it works:

- Push conventional commits to `main`.
- Release Please opens/updates a release PR with version/changelog updates.
- Merge the release PR to create a GitHub release and upload provider binaries + `checksums.txt`.

