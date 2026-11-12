# Administrator audit log kept one year, with the old and new values of each change.
resource "msexchangeserver_admin_audit_log_config" "this" {
  admin_audit_log_enabled        = true
  admin_audit_log_age_limit_days = 365
  log_level                      = "Verbose"
}
