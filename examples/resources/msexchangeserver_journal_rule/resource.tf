# Every message of the organization journaled to the archiving system.
resource "msexchangeserver_journal_rule" "all" {
  name                  = "Journal all mail"
  journal_email_address = "journal@archive.contoso.com"
  recipient             = ""
  scope                 = "Global"
  enabled               = true
}
