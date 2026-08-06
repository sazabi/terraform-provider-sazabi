resource "sazabi_script" "nightly_report" {
  project_id  = sazabi_project.production.id
  name        = "nightly-status-report"
  description = "Posts a nightly status summary to the on-call channel."
  content     = file("${path.module}/scripts/nightly-status-report.sh")
}
