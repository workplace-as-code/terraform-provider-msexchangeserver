# Restrict email settings for a partner domain
resource "msexchangeserver_remote_domain" "partner" {
  name        = "Contoso-Partner"
  domain_name = "partner.contoso.com"

  auto_reply_enabled            = false
  auto_forward_enabled          = false
  trusted_mail_outbound_enabled = true
  allowed_oof_type              = "None"
}

# Default wildcard remote domain with relaxed settings
resource "msexchangeserver_remote_domain" "internet" {
  name        = "Internet"
  domain_name = "*"

  auto_reply_enabled   = true
  auto_forward_enabled = false
  allowed_oof_type     = "External"
}
