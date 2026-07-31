# Terraform Provider for Sazabi

Declare Sazabi platform configuration — projects, status components, API keys, data source connections, and automation toggles — as code, backed by the [Sazabi public API](https://api.sazabi.com).

> **Status: pre-release.** Not yet published to the Terraform Registry.

## Usage

```hcl
terraform {
  required_providers {
    sazabi = {
      source  = "sazabi/sazabi"
      version = "~> 0.1"
    }
  }
}

provider "sazabi" {
  api_key         = var.sazabi_api_key         # or SAZABI_API_KEY
  organization_id = var.sazabi_organization_id # or SAZABI_ORGANIZATION_ID
  # base_url = "https://api.staging.sazabi.dev" # optional override for staging/region targeting
}
```

The `api_key` is a Sazabi **secret key** (`sazabi_secret_...`), created in the dashboard under Settings → API Keys or via the `sazabi_api_key` resource.

## Development

Requires Go (see `go.mod` for the version).

```sh
make build   # compile the provider
make test    # unit tests
make testacc # acceptance tests — needs TF_ACC=1, SAZABI_API_KEY, SAZABI_ORGANIZATION_ID against a sandbox org
make lint    # gofmt + go vet
```

## Design

The resource model, CRUD-honesty rules, and phasing live in the design doc:
[`docs/design/infrastructure/terraform-provider-v2/design.md`](https://github.com/sazabi/monorepo/blob/main/docs/design/infrastructure/terraform-provider-v2/design.md) in `sazabi/monorepo`. Implementation status is tracked in that design's `implementation/` directory.

Every resource maps 1:1 to public API operations defined in `packages/public-api-contracts` — the provider never invents capability the API does not have. Read-only concepts ship as `data` sources; partial-CRUD concepts model exactly the subset the API supports.
