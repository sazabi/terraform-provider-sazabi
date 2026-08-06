resource "sazabi_script" "nightly_report" {
  project_id = sazabi_project.production.id
  name       = "nightly-status-report"
  content    = file("${path.module}/scripts/nightly-status-report.sh")
}

resource "sazabi_automation" "nightly_report" {
  project_id      = sazabi_project.production.id
  name            = "nightly-status-report"
  script          = sazabi_script.nightly_report.name # or script_id = sazabi_script.nightly_report.id
  cron_expression = "17 9 * * *"
  timezone        = "UTC"
  timeout_seconds = 60
  enabled         = true
}
