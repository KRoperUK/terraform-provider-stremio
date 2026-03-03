# Resources and Data Sources

## Resource: `stremio_account`

Creates or imports a Stremio account.

### Example

```hcl
resource "stremio_account" "user" {
  email    = var.stremio_email
  password = var.stremio_password
}
```

### Import

```bash
terraform import stremio_account.user 'user@example.com:password'
```

## Resource: `stremio_addon_collection`

Manages full add-on collection (`transport_urls`) for an account.

### Example

```hcl
resource "stremio_addon_collection" "main" {
  transport_urls = [
    "https://v3-cinemeta.strem.io/manifest.json",
    "https://opensubtitles-v3.strem.io/manifest.json"
  ]
}
```

### Import

```bash
terraform import stremio_addon_collection.main addon-collection
```

### Multi-account pattern

```hcl
resource "stremio_addon_collection" "shared" {
  for_each = var.accounts

  email    = each.value.email
  password = each.value.password

  transport_urls = var.shared_transport_urls
}
```

## Data Source: `stremio_installed_addons`

Reads installed add-ons for authenticated account.

```hcl
data "stremio_installed_addons" "current" {}
```

## Data Source: `stremio_watch_history`

Reads watch history entries for authenticated account.

```hcl
data "stremio_watch_history" "recent" {
  limit = 25
}
```

## Data Source: `stremio_continue_watching`

Reads continue watching entries for authenticated account.

```hcl
data "stremio_continue_watching" "current" {
  limit = 25
}
```
