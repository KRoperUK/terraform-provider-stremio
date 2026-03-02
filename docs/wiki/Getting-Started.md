# Getting Started

## Prerequisites

- Go 1.22+
- Terraform 1.6+
- A Stremio account (or credentials to create one)

## Local Setup

1. Clone the repo.
2. Copy `.env.example` to `.env` and fill credentials.
3. Build provider:

```bash
go build -o terraform-provider-stremio
```

## Basic Example

Use the basic example:

```bash
./scripts/plan.sh -no-color
./scripts/apply.sh -auto-approve
```

## Multi-Account Example

1. Copy `examples/multi-account/terraform.tfvars.example` to `examples/multi-account/terraform.tfvars`.
2. Fill account credentials and shared add-on URLs.
3. Run:

```bash
./scripts/plan-multi.sh -no-color
./scripts/apply-multi.sh -auto-approve
```
