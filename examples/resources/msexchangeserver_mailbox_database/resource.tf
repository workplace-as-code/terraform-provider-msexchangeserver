# Default quotas of the mailboxes of DB01, and 30 days to recover deleted items.
resource "msexchangeserver_mailbox_database" "db01" {
  name                           = "DB01"
  issue_warning_quota_mb         = 47104
  prohibit_send_quota_mb         = 49152
  prohibit_send_receive_quota_mb = 51200
  deleted_item_retention_days    = 30
}
