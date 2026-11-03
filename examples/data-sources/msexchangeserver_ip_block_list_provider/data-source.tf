data "msexchangeserver_ip_block_list_provider" "this" {
  name = "Spamhaus"
}

output "lookup_domain" {
  description = "Lookup domain of the ip block list provider."
  value       = data.msexchangeserver_ip_block_list_provider.this.lookup_domain
}
