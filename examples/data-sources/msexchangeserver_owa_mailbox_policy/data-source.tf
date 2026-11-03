data "msexchangeserver_owa_mailbox_policy" "this" {
  name = "OwaMailboxPolicy-Default"
}

output "instant_messaging_enabled" {
  description = "Whether the default Outlook on the web policy allows instant messaging."
  value       = data.msexchangeserver_owa_mailbox_policy.this.instant_messaging_enabled
}
