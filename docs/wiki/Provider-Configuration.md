# Provider Configuration

```hcl
provider "stremio" {
  base_url = "https://api.strem.io"
  email    = var.stremio_email
  password = var.stremio_password
}
```

## Arguments

- `base_url` (optional): Stremio API endpoint.
- `email` (optional): Provider-level login email.
- `password` (optional, sensitive): Provider-level login password.

## Notes

- Resource-level credentials can override provider-level credentials where supported.
- For multi-account management, prefer per-resource `email`/`password` on `stremio_addon_collection`.
