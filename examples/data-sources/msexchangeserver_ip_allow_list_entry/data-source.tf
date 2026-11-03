data "msexchangeserver_ip_allow_list_entry" "this" {
  name = "192.0.2.0/24"
}

output "comment" {
  description = "Comment of the ip allow list entry."
  value       = data.msexchangeserver_ip_allow_list_entry.this.comment
}
