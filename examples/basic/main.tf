terraform {
  required_providers {
    stremio = {
      source = "kroperuk/stremio"
    }
  }
}

provider "stremio" {
  base_url = "https://api.strem.io"
  email    = var.stremio_email
  password = var.stremio_password
}

resource "stremio_account" "user" {
  email    = var.stremio_email
  password = var.stremio_password
}

resource "stremio_addon_collection" "managed" {
  depends_on = [stremio_account.user]

  transport_urls = [
    "https://v3-cinemeta.strem.io/manifest.json",
    "https://opensubtitles-v3.strem.io/manifest.json",
  ]
}

data "stremio_installed_addons" "installed" {
  depends_on = [stremio_addon_collection.managed]
}

output "addons" {
  value = data.stremio_installed_addons.installed.addons
}
