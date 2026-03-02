#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="${REPO_ROOT}/.env"
EXAMPLE_DIR="${REPO_ROOT}/examples/basic"

if [[ ! -f "${ENV_FILE}" ]]; then
  echo "Missing .env file at ${ENV_FILE}" >&2
  exit 1
fi

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

STREMIO_EMAIL="$(read_env_var "STREMIO_EMAIL")"
STREMIO_PASSWORD="$(read_env_var "STREMIO_PASSWORD")"

echo "Building local Terraform provider..."
cd "${REPO_ROOT}"
go build -o terraform-provider-stremio

if ! command -v terraform >/dev/null 2>&1; then
  echo "Terraform CLI not found. Falling back to direct Stremio API register call."

  payload="{\"email\":\"${STREMIO_EMAIL}\",\"password\":\"${STREMIO_PASSWORD}\"}"
  register_response="$(curl -sS -X POST \
    -H "content-type: application/json" \
    --data "${payload}" \
    "https://api.strem.io/api/register")"

  if echo "${register_response}" | grep -q '"error"'; then
    if ! echo "${register_response}" | grep -q '"error":null'; then
      echo "Register request failed: ${register_response}" >&2
      exit 1
    fi
  fi

  echo "Account registration request completed."
  exit 0
fi

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
export TF_VAR_stremio_email="${STREMIO_EMAIL}"
export TF_VAR_stremio_password="${STREMIO_PASSWORD}"

echo "Applying Terraform to create the account..."
set +e
APPLY_OUTPUT="$(terraform -chdir="${EXAMPLE_DIR}" apply -auto-approve 2>&1)"
APPLY_EXIT=$?
set -e

if [[ ${APPLY_EXIT} -eq 0 ]]; then
  echo "${APPLY_OUTPUT}"
  echo "Terraform apply completed successfully."
  exit 0
fi

if echo "${APPLY_OUTPUT}" | grep -q "existingUser"; then
  echo "Account already exists. Importing into Terraform state..."
  terraform -chdir="${EXAMPLE_DIR}" import stremio_account.user "${STREMIO_EMAIL}:${STREMIO_PASSWORD}"
  terraform -chdir="${EXAMPLE_DIR}" apply -auto-approve
  echo "Existing account imported and Terraform apply completed."
  exit 0
fi

echo "${APPLY_OUTPUT}" >&2
exit ${APPLY_EXIT}
