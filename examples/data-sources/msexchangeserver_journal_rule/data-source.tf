data "msexchangeserver_journal_rule" "all" {
  name = "Journal all mail"
}

output "journal_address" {
  description = "Address that receives the journal reports of the rule."
  value       = data.msexchangeserver_journal_rule.all.journal_email_address
}
