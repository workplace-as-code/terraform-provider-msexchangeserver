data "msexchangeserver_mailbox" "helpdesk" {
  identity = "helpdesk@contoso.com"
}

output "helpdesk_address" {
  description = "Primary SMTP address of the helpdesk mailbox."
  value       = data.msexchangeserver_mailbox.helpdesk.primary_smtp_address
}
