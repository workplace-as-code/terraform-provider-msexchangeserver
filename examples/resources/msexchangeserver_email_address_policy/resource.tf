# Sales mailboxes get a first.last address in the sales subdomain. The existing
# Sales mailboxes get it too: apply_to_recipients runs Update-EmailAddressPolicy.
resource "msexchangeserver_email_address_policy" "sales" {
  name                            = "Sales Team"
  enabled_email_address_templates = ["SMTP:%g.%s@sales.contoso.com"]
  included_recipients             = "MailboxUsers"
  conditional_department          = "Sales"
  priority                        = 1
  apply_to_recipients             = true
}
