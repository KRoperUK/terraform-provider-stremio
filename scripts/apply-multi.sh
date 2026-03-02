#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export EXAMPLE_DIR="$(cd "${SCRIPT_DIR}/../examples/multi-account" && pwd)"
source "${SCRIPT_DIR}/_terraform-env.sh"

cd "${REPO_ROOT}"
go build -o terraform-provider-stremio

prepare_terraform_env

terraform -chdir="${EXAMPLE_DIR}" apply "$@"
