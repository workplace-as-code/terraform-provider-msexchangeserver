# Reject mail to recipients that do not exist.
resource "msexchangeserver_recipient_filter_config" "this" {
  recipient_validation_enabled = true
}
