# Hardened OWA policy for standard users — disables risky features
resource "msexchangeserver_owa_mailbox_policy" "standard" {
  name = "Standard-OWA"

  # Keep productivity features enabled
  attachments_enabled           = true
  recover_deleted_items_enabled = true
  rules_enabled                 = true
  signatures_enabled            = true

  # Disable features that increase the attack surface
  instant_messaging_enabled       = false
  active_sync_integration_enabled = false
  public_folders_enabled          = false
  theme_selection_enabled         = false

  # Office Online document viewing
  wac_viewing_on_private_computers_enabled = true
  wac_viewing_on_public_computers_enabled  = false

  # Security headers and encoding
  web_parts_frame_options_type = "SameOrigin"
  outbound_charset             = "AlwaysUTF8"

  # Block dangerous attachment types in OWA
  blocked_file_types = ["exe", "bat", "cmd", "ps1", "vbs", "js", "jar", "scr"]
}

# Permissive OWA policy for IT administrators
resource "msexchangeserver_owa_mailbox_policy" "it_admins" {
  name = "IT-Admins-OWA"

  attachments_enabled             = true
  recover_deleted_items_enabled   = true
  rules_enabled                   = true
  signatures_enabled              = true
  instant_messaging_enabled       = true
  active_sync_integration_enabled = true
  public_folders_enabled          = true
  theme_selection_enabled         = true

  wac_viewing_on_private_computers_enabled = true
  wac_viewing_on_public_computers_enabled  = true

  web_parts_frame_options_type = "SameOrigin"
  outbound_charset             = "AlwaysUTF8"
}
