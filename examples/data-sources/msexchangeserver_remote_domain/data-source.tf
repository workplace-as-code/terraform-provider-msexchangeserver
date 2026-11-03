data "msexchangeserver_remote_domain" "this" {
  name = "Default"
}

output "auto_forward_enabled" {
  description = "Whether automatic forwarding is allowed to external domains."
  value       = data.msexchangeserver_remote_domain.this.auto_forward_enabled
}
