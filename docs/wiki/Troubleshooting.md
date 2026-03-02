# Troubleshooting

## `Invalid resource type`

Cause: Terraform is not using local dev override provider.

Fix:

- Use helper scripts (`./scripts/plan.sh`, `./scripts/apply.sh`) or
- Set `TF_CLI_CONFIG_FILE` with `dev_overrides` for `kroperuk/stremio`.

## `existingUser` during account creation

Cause: Account already exists.

Fix:

- Import existing account (`email:password`) or
- Skip account creation in multi-account mode with `create_accounts = false`.

## `not authenticated`

Cause: No credentials available for provider/resource.

Fix:

- Configure provider `email`/`password`, or
- Set per-resource credentials on `stremio_addon_collection`.

## Addon fetch/login issues in Stremio client

Cause: Invalid/unreachable add-on URLs or stale client cache.

Fix:

- Verify each `transport_urls` endpoint is reachable.
- Re-apply addon collection.
- Log out/in in client apps to refresh local add-on cache.
