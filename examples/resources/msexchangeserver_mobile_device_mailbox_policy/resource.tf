# Corporate BYOD strict policy
resource "msexchangeserver_mobile_device_mailbox_policy" "byod_strict" {
  name                           = "BYOD Strict"
  password_enabled               = true
  alphanumeric_password_required = true
  min_password_length            = 8
  max_password_failed_attempts   = 5
  max_inactivity_time_lock       = "00:05:00"
  device_encryption_enabled      = true
  require_device_encryption      = true
  allow_camera                   = false
  allow_text_messaging           = false
  allow_bluetooth                = "HandsfreeOnly"
}

# Default permissive policy
resource "msexchangeserver_mobile_device_mailbox_policy" "default" {
  name       = "Default"
  is_default = true
}
