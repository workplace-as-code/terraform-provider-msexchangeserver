data "msexchangeserver_global_address_list" "default" {
  name = "Default Global Address List"
}

output "default_gal_filter" {
  description = "Filter of the default GAL, which cannot be changed."
  value       = data.msexchangeserver_global_address_list.default.recipient_filter
}
