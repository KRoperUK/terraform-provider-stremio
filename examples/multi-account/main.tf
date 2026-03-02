terraform {
  required_providers {
    stremio = {
      source = "kroperuk/stremio"
    }
  }
}

provider "stremio" {
  base_url = "https://api.strem.io"
}

resource "stremio_account" "accounts" {
  for_each = var.create_accounts ? var.accounts : {}

  email    = each.value.email
  password = each.value.password
}

resource "stremio_addon_collection" "shared" {
  for_each = var.accounts

  email    = each.value.email
  password = each.value.password

  transport_urls = var.shared_transport_urls
}

output "managed_accounts" {
  value = keys(var.accounts)
}
