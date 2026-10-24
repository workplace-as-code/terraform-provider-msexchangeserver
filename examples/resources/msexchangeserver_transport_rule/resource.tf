# Block executable attachments
resource "msexchangeserver_transport_rule" "block_executables" {
  name     = "Block executable attachments"
  priority = 1
  enabled  = true

  attachment_is_executable = true

  action         = "Reject"
  reject_message = "Executable attachments are not permitted by security policy."
}

# Prevent external senders from spoofing the internal domain
resource "msexchangeserver_transport_rule" "block_spoofed_domain" {
  name     = "Block spoofed internal domain"
  priority = 2
  enabled  = true

  from_address_contains = ["@contoso.com"]

  action         = "Reject"
  reject_message = "External senders cannot use internal domain addresses."
}

# Send messages with social-engineering keywords in the subject to the Junk Email folder
resource "msexchangeserver_transport_rule" "junk_suspicious_subjects" {
  name     = "Junk suspicious subject keywords"
  priority = 10
  enabled  = true

  subject_contains = ["invoice urgent", "wire transfer", "click here immediately"]

  action = "MoveToJmf"
}
