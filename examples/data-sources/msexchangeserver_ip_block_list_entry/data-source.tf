data "msexchangeserver_ip_block_list_entry" "this" {
  name = "198.51.100.7"
}

output "expiration_time" {
  description = "Expiration time of the ip block list entry."
  value       = data.msexchangeserver_ip_block_list_entry.this.expiration_time
}
