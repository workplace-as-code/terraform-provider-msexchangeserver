data "msexchangeserver_receive_connector" "this" {
  name = "EX01\\Application relay"
}

output "remote_ip_ranges" {
  description = "Servers allowed to relay."
  value       = data.msexchangeserver_receive_connector.this.remote_ip_ranges
}
