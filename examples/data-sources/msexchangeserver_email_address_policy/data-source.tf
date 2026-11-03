data "msexchangeserver_email_address_policy" "this" {
  name = "Default Policy"
}

output "enabled_email_address_templates" {
  description = "Address templates of the default email address policy."
  value       = data.msexchangeserver_email_address_policy.this.enabled_email_address_templates
}
