# Message limits of the organization, postmaster address and shadow redundancy.
resource "msexchangeserver_transport_config" "this" {
  max_send_size_mb             = 35
  max_receive_size_mb          = 35
  max_recipient_envelope_limit = 500

  external_postmaster_address = "postmaster@contoso.com"
  internal_smtp_servers       = ["10.0.0.25"]

  shadow_redundancy_enabled    = true
  safety_net_hold_time_minutes = 2880
}
