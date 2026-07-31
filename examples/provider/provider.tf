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

  # base_url override for staging/region targeting (no /v1 suffix):
  # base_url = "https://api.staging.sazabi.dev"
}

variable "sazabi_api_key" {
  type      = string
  sensitive = true
}

variable "sazabi_organization_id" {
  type = string
}
