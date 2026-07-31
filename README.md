# Terraform Provider for Sazabi

Declare Sazabi platform configuration — projects, status components, API keys, data source connections, and automation toggles — as code, backed by the [Sazabi public API](https://api.sazabi.com).

> **Status: pre-release.** Private repo, not yet published to the Terraform Registry.

## Configuration

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
  # base_url = "https://api.staging.sazabi.dev" # optional override (no /v1 suffix), or SAZABI_API_BASE_URL
}
```

The `api_key` is a Sazabi **secret key** (`sazabi_secret_...`), created in the dashboard under Settings → API Keys or via the `sazabi_api_key` resource. Auth precedence mirrors the CLI: explicit provider block, then environment variable, then a clear failure. The provider is non-interactive by design — no OAuth login path.

## Resources and data sources

Every resource maps 1:1 to public API operations defined in `packages/public-api-contracts` (in `sazabi/monorepo`) — the provider never models capability the API does not have. Partial-CRUD concepts expose exactly the subset the API supports; OAuth-backed concepts ship as read-only `data` sources.

| Name | Kind | CRUD surface | Notes |
|------|------|--------------|-------|
| `sazabi_project` | resource | Full CRUD | Update is rename-only; `organization_id`/`region` force replacement. Delete rejects the org's last active project. Requires `projects.update`/`.delete` (monorepo #12366) deployed. |
| `sazabi_status_component` | resource | Register / deregister | Register is an upsert by name — creating an existing name adopts it. Re-register updates the description; clearing it forces replacement. Destroy soft-deletes. |
| `sazabi_api_key` | resource | Full CRUD | Plaintext `value` returned once at create, stored sensitive in state; empty on import. |
| `sazabi_data_source_connection` | resource | Create / read / delete | No update endpoint: credential rotation is destroy-and-recreate. `metadata` is a sensitive `map(string)`, write-only server-side (never drift-checked). |
| `sazabi_data_source_stream` | resource | Create / read / delete | Async provisioning; volatile status fields not tracked. Some sources mint a one-time per-stream `public_key`. |
| `sazabi_automation` | resource | Toggle only | Adopts an existing automation by `automation_id`; the API cannot create, edit, or delete automations. Destroy leaves the automation untouched. |
| `sazabi_public_key_log_forwarding` | resource | Ensure / deactivate | Upsert keyed by project; plaintext value recoverable on every apply. Destroy soft-disables. |
| `data.sazabi_integration_connection` | data source | Read-only | Integrations connect via interactive OAuth, which Terraform cannot drive. |
| `data.sazabi_mcp_connector` | data source | Read-only | Same OAuth constraint. |

### Example

```hcl
resource "sazabi_project" "production" {
  name   = "Production"
  region = "us-west-2"
}

resource "sazabi_status_component" "checkout_api" {
  project_id  = sazabi_project.production.id
  name        = "checkout-api"
  description = "Customer-facing checkout service"
}

resource "sazabi_api_key" "ci_agent" {
  name       = "ci-agent"
  project_id = sazabi_project.production.id
}

resource "sazabi_data_source_connection" "vercel_logs" {
  project_id       = sazabi_project.production.id
  data_source_type = "vercel"
  display_name     = "Production Vercel logs"

  metadata = {
    team_id = var.vercel_team_id
    token   = var.vercel_api_token
  }
}

resource "sazabi_automation" "nightly_status_report" {
  automation_id = var.nightly_status_report_automation_id # created in the dashboard
  project_id    = sazabi_project.production.id
  enabled       = true
}
```

Every resource supports `terraform import` (by ID). Secret-valued attributes (`sazabi_api_key.value`, connection/stream public keys) are unrecoverable on import and recorded empty — the API returns them exactly once at creation.

## Development

Requires Go (see `go.mod` for the version; use `mise exec go@<version> --` if you manage toolchains with mise).

```sh
make build   # compile the provider
make test    # unit tests (no network, no credentials)
make testacc # acceptance tests — see below
make lint    # gofmt + go vet
make fmt     # gofmt -w
```

### Acceptance tests

Acceptance tests exercise a **real Sazabi organization** — use a dedicated sandbox org, never a customer or production org.

```sh
export SAZABI_API_KEY=sazabi_secret_...        # sandbox org secret key
export SAZABI_ORGANIZATION_ID=...              # sandbox org
export SAZABI_TEST_PROJECT_ID=...              # existing project, for status component tests
make testacc                                   # sets TF_ACC=1
```

The sandbox org should keep at least one persistent project beyond what tests create: the API rejects deleting an organization's last active project.

### Local provider override

To run an unreleased build against real Terraform configs, use a [dev override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) in `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "sazabi/sazabi" = "/path/to/terraform-provider-sazabi"  # directory containing the built binary
  }
  direct {}
}
```

Then `go build` and run `terraform plan` in your config directory (skip `terraform init` for the overridden provider).

## Releasing

`.goreleaser.yml` and `terraform-registry-manifest.json` follow HashiCorp's provider scaffolding. Terraform Registry publishing is deliberately deferred: it requires making this repo public, choosing an OSI-approved license, and provisioning a GPG signing key. Until then, releases are internal only.

## Design

The resource model, CRUD-honesty rules, and phasing live in the design doc:
[`docs/design/infrastructure/terraform-provider-v2/design.md`](https://github.com/sazabi/monorepo/blob/main/docs/design/infrastructure/terraform-provider-v2/design.md) in `sazabi/monorepo`. Implementation status is tracked in that design's `implementation/` directory. The design is a frozen historical artifact; this README and the code are the living documentation.
