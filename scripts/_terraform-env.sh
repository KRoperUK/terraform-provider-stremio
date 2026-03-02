#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${REPO_ROOT}/.env"
EXAMPLE_DIR_DEFAULT="${REPO_ROOT}/examples/basic"
EXAMPLE_DIR="${EXAMPLE_DIR:-${EXAMPLE_DIR_DEFAULT}}"

read_env_var() {
  local key="$1"
  local value
  value="$(grep -E "^[[:space:]]*${key}[[:space:]]*=" "${ENV_FILE}" | tail -n1 | sed -E 's/^[^=]*=[[:space:]]*//; s/^["'"'"']//; s/["'"'"']$//' | tr -d '\r')"
  if [[ -z "${value}" ]]; then
    echo "Missing or empty ${key} in ${ENV_FILE}" >&2
    exit 1
  fi
  printf '%s' "${value}"
}

prepare_terraform_env() {
  if [[ ! -f "${ENV_FILE}" ]]; then
    echo "Missing .env file at ${ENV_FILE}" >&2
    exit 1
  fi

  if ! command -v terraform >/dev/null 2>&1; then
    echo "Terraform CLI not found in PATH." >&2
    exit 1
  fi

  local email password
  email="$(read_env_var "STREMIO_EMAIL")"
  password="$(read_env_var "STREMIO_PASSWORD")"

  TF_CLI_CONFIG_FILE="$(mktemp)"
  trap 'rm -f "${TF_CLI_CONFIG_FILE}"' EXIT

  cat > "${TF_CLI_CONFIG_FILE}" <<EOF
provider_installation {
  dev_overrides {
    "kroperuk/stremio" = "${REPO_ROOT}"
  }
  direct {}
}
EOF

  export TF_CLI_CONFIG_FILE
  export TF_VAR_stremio_email="${email}"
  export TF_VAR_stremio_password="${password}"
}
