variable "accounts" {
  description = "Map of account entries to manage. Key is any label, value contains login credentials."
  type = map(object({
    email    = string
    password = string
  }))
}

variable "shared_transport_urls" {
  description = "Same addon transport URLs applied to every account in var.accounts."
  type        = list(string)
  default = [
    "https://v3-cinemeta.strem.io/manifest.json",
    "https://opensubtitles-v3.strem.io/manifest.json",
  ]
}
