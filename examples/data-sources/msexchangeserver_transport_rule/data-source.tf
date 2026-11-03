data "msexchangeserver_transport_rule" "this" {
  name = "Block executable attachments"
}

output "enabled" {
  description = "Whether the rule is enabled."
  value       = data.msexchangeserver_transport_rule.this.enabled
}
