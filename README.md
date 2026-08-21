# Terraform Provider for Sazabi

Declare Sazabi platform configuration — projects, components, API keys, log sources, scripts, and scheduled automations — as code, backed by the [Sazabi public API](https://api.sazabi.com).

> **Source model:** The provider's Go source is maintained in Sazabi's private monorepo. This repository distributes the provider as **pre-built, signed binaries only** via the [Terraform Registry](https://registry.terraform.io/providers/sazabi/sazabi/latest). There is no publicly visible Go source here.

## Installation

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

The `api_key` is a Sazabi **secret key** (`sazabi_secret_...`), created in the dashboard under Settings → API Keys or via the `sazabi_api_key` resource. Auth precedence mirrors the CLI: explicit provider block, then environment variable, then a clear failure. The provider is non-interactive — no OAuth login path.

## Resources and data sources

Every resource maps 1:1 to public API operations. Partial-CRUD concepts expose exactly the subset the API supports; OAuth-backed concepts ship as read-only `data` sources.

| Name | Kind | CRUD surface | Notes |
|------|------|--------------|-------|
| `sazabi_project` | resource | Full CRUD | Update is rename-only; `organization_id`/`region` force replacement. Delete rejects the org's last active project. |
| `sazabi_component` | resource | Register / deregister | Register is an upsert by name — creating an existing name adopts it. Re-register updates the description; clearing it forces replacement. Destroy soft-deletes. |
| `sazabi_api_key` | resource | Full CRUD | Plaintext `value` returned once at create, stored sensitive in state; empty on import. |
| `sazabi_log_source` | resource | Create / read / delete | No update endpoint: credential rotation is destroy-and-recreate. `metadata` is a sensitive `map(string)`, write-only server-side. |
| `sazabi_log_stream` | resource | Create / read / delete | Async provisioning; volatile status fields not tracked. Some sources mint a one-time per-stream `public_key`. |
| `sazabi_script` | resource | Full CRUD | A durable bash script keyed by name within a project. Renaming forces replacement; destroy soft-deletes. |
| `sazabi_automation` | resource | Create / read / update + enable/disable | Scheduled automation that runs a `sazabi_script` on a cron schedule. Name/description/schedule update in place; the script it runs is immutable (forces replacement). The public API has no delete-automation operation, so destroy **disables** the automation and removes it from state — it is not deleted server-side. |
| `sazabi_public_key_log_forwarding` | resource | Ensure / deactivate | Upsert keyed by project; plaintext value recoverable on every apply. Destroy soft-disables. |
| `data.sazabi_integration_connection` | data source | Read-only | Integrations connect via interactive OAuth, which Terraform cannot drive. |
| `data.sazabi_mcp_connector` | data source | Read-only | Same OAuth constraint. |

### Example

```hcl
resource "sazabi_project" "production" {
  name   = "Production"
  region = "us-west-2"
}

resource "sazabi_component" "checkout_api" {
  project_id  = sazabi_project.production.id
  name        = "checkout-api"
  description = "Customer-facing checkout service"
}

resource "sazabi_api_key" "ci_agent" {
  name       = "ci-agent"
  project_id = sazabi_project.production.id
}

resource "sazabi_log_source" "vercel_logs" {
  project_id = sazabi_project.production.id
  source     = "vercel"
  name       = "Production Vercel logs"

  metadata = {
    team_id = var.vercel_team_id
    token   = var.vercel_api_token
  }
}

resource "sazabi_script" "nightly_report" {
  project_id = sazabi_project.production.id
  name       = "nightly-status-report"
  content    = file("${path.module}/scripts/nightly-status-report.sh")
}

resource "sazabi_automation" "nightly_status_report" {
  project_id      = sazabi_project.production.id
  name            = "nightly-status-report"
  script          = sazabi_script.nightly_report.name
  cron_expression = "17 9 * * *"
  timezone        = "UTC"
  timeout_seconds = 60
  enabled         = true
}
```

Every resource supports `terraform import`. Most import by ID; `sazabi_script` imports by name (`terraform import sazabi_script.nightly_report nightly-status-report`, or `projectId/name` to pin the project). Secret-valued attributes (`sazabi_api_key.value`, log-source/log-stream public keys) are unrecoverable on import and recorded empty — the API returns them exactly once at creation.

### Breaking change: `sazabi_status_component` is now `sazabi_component`

Sazabi renamed "status components" to plain **components** across the platform. The resource type follows: `sazabi_status_component` → `sazabi_component`, same schema and behavior, no aliases. `terraform state mv` cannot move state across resource types, so existing configurations rename the type in HCL, drop the old state entry, and re-import:

```sh
terraform state rm 'sazabi_status_component.example'
terraform import 'sazabi_component.example' <component-id>
```

### Breaking change: `sazabi_automation` is now full CRUD

The previous resource modeled only an enable/disable toggle, adopting an existing automation by a required `automation_id`. That resource has been reworked into a full-CRUD resource that **creates and owns** the automation:

- `automation_id` is **removed**. The automation is created by Terraform; its server-assigned ID is the computed `id`.
- New required/optional config: `name`, `script` **xor** `script_id`, `cron_expression`, `timezone`, `timeout_seconds`, `enabled`, `description`.
- The script an automation runs is immutable per the API's update contract — changing `script`/`script_id` forces replacement.
- **Destroy disables, it does not delete.** The public API has no delete-automation operation.

To keep managing an automation that was previously adopted by `automation_id`, drop the old resource from state and re-import it into the new resource by its ID, or recreate the script + automation in HCL.

## Releases

Releases are published to the [Terraform Registry](https://registry.terraform.io/providers/sazabi/sazabi/latest) as signed binaries. Each GitHub release in this repository contains the binary archives and the GPG signature file — no Go source is included.

Binaries are signed with Sazabi's GPG key. The Terraform Registry verifies signatures automatically; no manual verification step is needed for normal `terraform init` usage.

## Support and feedback

For questions, bug reports, or feature requests, contact [support@sazabi.ai](mailto:support@sazabi.ai).

## License

[Apache 2.0](LICENSE)
