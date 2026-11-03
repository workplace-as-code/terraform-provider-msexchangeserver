# Reject at SCL 6 and above, quarantine nothing.
resource "msexchangeserver_content_filter_config" "this" {
  scl_reject_enabled   = true
  scl_reject_threshold = 6
}
