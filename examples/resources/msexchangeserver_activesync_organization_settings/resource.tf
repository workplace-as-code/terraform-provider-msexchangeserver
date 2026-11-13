# Unknown devices wait in quarantine until an administrator allows them.
resource "msexchangeserver_activesync_organization_settings" "this" {
  default_access_level  = "Quarantine"
  admin_mail_recipients = ["mobile-admins@contoso.com"]
  user_mail_insert      = "Your device is waiting for approval by the IT service desk."
}
