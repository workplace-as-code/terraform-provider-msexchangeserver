data "msexchangeserver_mobile_device_mailbox_policy" "this" {
  name = "Default"
}

output "password_enabled" {
  description = "Whether the default mobile device policy requires a password."
  value       = data.msexchangeserver_mobile_device_mailbox_policy.this.password_enabled
}
