variable "stremio_email" {
  type = string
}

variable "stremio_password" {
  type      = string
  sensitive = true
}
