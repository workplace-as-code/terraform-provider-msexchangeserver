data "msexchangeserver_remote_mailbox" "helpdesk" {
  identity = "helpdesk@contoso.com"
}

output "helpdesk_routing_address" {
  description = "Where on-premises mail to the helpdesk is routed in Exchange Online."
  value       = data.msexchangeserver_remote_mailbox.helpdesk.remote_routing_address
}
