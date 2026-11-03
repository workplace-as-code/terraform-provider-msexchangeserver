data "msexchangeserver_send_connector" "this" {
  name = "To internet via gateways"
}

output "address_spaces" {
  description = "Domains routed through the connector."
  value       = data.msexchangeserver_send_connector.this.address_spaces
}
