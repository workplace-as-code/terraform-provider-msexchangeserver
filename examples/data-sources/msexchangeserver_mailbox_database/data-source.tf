data "msexchangeserver_mailbox_database" "db01" {
  name = "DB01"
}

output "db01_send_quota_mb" {
  description = "Mailbox size, in MB, at which users of DB01 can no longer send."
  value       = data.msexchangeserver_mailbox_database.db01.prohibit_send_quota_mb
}
