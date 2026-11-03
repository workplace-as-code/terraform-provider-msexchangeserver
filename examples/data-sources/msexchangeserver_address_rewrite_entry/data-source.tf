data "msexchangeserver_address_rewrite_entry" "this" {
  name = "Fabrikam to Contoso"
}

output "external_address" {
  description = "External address of the address rewrite entry."
  value       = data.msexchangeserver_address_rewrite_entry.this.external_address
}
