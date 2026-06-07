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
  for_each = var.accounts

  email    = each.value.email
  password = each.value.password

  # Add-on collection management is built into the account resource.
  transport_urls = var.shared_transport_urls
}

output "managed_accounts" {
  value = keys(var.accounts)
}
