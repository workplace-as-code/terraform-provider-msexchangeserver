resource "msexchangeserver_organization_config" "org" {
  read_tracking_enabled                      = false
  mail_tips_all_tips_enabled                 = true
  mail_tips_external_recipients_tips_enabled = true
  mail_tips_large_audience_threshold         = 25
  oauth2_client_profile_enabled              = true
}
